// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

// DeployedAppMetadata wraps the Metadata field of a deployed
// application StackSet.
type DeployedAppMetadata struct {
	Metadata AppResourcesConfig `yaml:"Metadata"`
}

// AppResourcesConfig is a configuration for a deployed Application
// StackSet.
type AppResourcesConfig struct {
	Accounts []string `yaml:"Accounts,flow"`
	Services []string `yaml:"Services,flow"`
	App      string   `yaml:"App"`
	Version  int      `yaml:"Version"`
}

// AppStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type AppStackConfig struct {
	*deploy.CreateAppInput
	parser template.ReadParser
}

// AppRegionalResources represent application resources that are regional.
type AppRegionalResources struct {
	Region         string            // The region these resources are in.
	KMSKeyARN      string            // A KMS Key ARN for encrypting Pipeline artifacts.
	S3Bucket       string            // S3 bucket for Pipeline artifacts.
	RepositoryURLs map[string]string // The image repository URLs by service name.
}

const (
	appTemplatePath               = "app/app.yml"
	appResourcesTemplatePath      = "app/cf.yml"
	appAdminRoleParamName         = "AdminRoleName"
	appExecutionRoleParamName     = "ExecutionRoleName"
	appDNSDelegationRoleParamName = "DNSDelegationRoleName"
	appOutputKMSKey               = "KMSKeyARN"
	appOutputS3Bucket             = "PipelineBucket"
	appOutputECRRepoPrefix        = "ECRRepo"
	appDNSDelegatedAccountsKey    = "AppDNSDelegatedAccounts"
	appDomainNameKey              = "AppDomainName"
	appNameKey                    = "AppName"
	appDNSDelegationRoleName      = "DNSDelegationRole"
)

var cfTemplateFunctions = map[string]interface{}{
	"logicalIDSafe": template.ReplaceDashesFunc,
}

// AppConfigFrom takes a template file and extracts the metadata block,
// and parses it into an AppStackConfig
func AppConfigFrom(template *string) (*AppResourcesConfig, error) {
	resourceConfig := DeployedAppMetadata{}
	err := yaml.Unmarshal([]byte(*template), &resourceConfig)
	return &resourceConfig.Metadata, err
}

// NewAppStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up an environment.
func NewAppStackConfig(in *deploy.CreateAppInput) *AppStackConfig {
	return &AppStackConfig{
		CreateAppInput: in,
		parser:         template.New(),
	}
}

// Template returns the environment CloudFormation template.
func (c *AppStackConfig) Template() (string, error) {
	content, err := c.parser.Read(appTemplatePath)
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// ResourceTemplate generates a StackSet template with all the Application-wide resources (ECR Repos, KMS keys, S3 buckets)
func (c *AppStackConfig) ResourceTemplate(config *AppResourcesConfig) (string, error) {
	// Sort the account IDs and Services so that the template we generate is deterministic
	sort.Strings(config.Accounts)
	sort.Strings(config.Services)

	content, err := c.parser.Parse(appResourcesTemplatePath, struct {
		*AppResourcesConfig
		ServiceTagKey string
	}{
		config,
		ServiceTagKey,
	}, template.WithFuncs(cfTemplateFunctions))
	if err != nil {
		return "", err
	}
	return content.String(), err
}

// Parameters returns a list of parameters which accompany the app CloudFormation template.
func (c *AppStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(appAdminRoleParamName),
			ParameterValue: aws.String(c.stackSetAdminRoleName()),
		},
		{
			ParameterKey:   aws.String(appExecutionRoleParamName),
			ParameterValue: aws.String(c.StackSetExecutionRoleName()),
		},
		{
			ParameterKey:   aws.String(appDNSDelegatedAccountsKey),
			ParameterValue: aws.String(strings.Join(c.dnsDelegationAccounts(), ",")),
		},
		{
			ParameterKey:   aws.String(appDomainNameKey),
			ParameterValue: aws.String(c.DomainName),
		},
		{
			ParameterKey:   aws.String(appNameKey),
			ParameterValue: aws.String(c.Name),
		},
		{
			ParameterKey:   aws.String(appDNSDelegationRoleParamName),
			ParameterValue: aws.String(dnsDelegationRoleName(c.Name)),
		},
	}, nil
}

// Tags returns the tags that should be applied to the Application CloudFormation stack.
func (c *AppStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(c.AdditionalTags, map[string]string{
		AppTagKey: c.Name,
	})
}

// StackName returns the name of the CloudFormation stack (based on the application name).
func (c *AppStackConfig) StackName() string {
	return fmt.Sprintf("%s-infrastructure-roles", c.Name)
}

// StackSetName returns the name of the CloudFormation StackSet (based on the application name).
func (c *AppStackConfig) StackSetName() string {
	return fmt.Sprintf("%s-infrastructure", c.Name)
}

// StackSetDescription returns the description of the StackSet for application resources.
func (c *AppStackConfig) StackSetDescription() string {
	return "ECS CLI Application Resources (ECR repos, KMS keys, S3 buckets)"
}

func (c *AppStackConfig) stackSetAdminRoleName() string {
	return fmt.Sprintf("%s-adminrole", c.Name)
}

// StackSetAdminRoleARN returns the role ARN of the role used to administer the Application
// StackSet.
func (c *AppStackConfig) StackSetAdminRoleARN() string {
	//TODO find a partition-neutral way to construct this ARN
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", c.AccountID, c.stackSetAdminRoleName())
}

// StackSetExecutionRoleName returns the role name of the role used to actually create
// Application resources.
func (c *AppStackConfig) StackSetExecutionRoleName() string {
	return fmt.Sprintf("%s-executionrole", c.Name)
}

func (c *AppStackConfig) dnsDelegationAccounts() []string {
	accounts := append(c.CreateAppInput.DNSDelegationAccounts, c.CreateAppInput.AccountID)
	accountIDs := make(map[string]bool)
	var uniqueAccountIDs []string
	for _, entry := range accounts {
		if _, value := accountIDs[entry]; !value {
			accountIDs[entry] = true
			uniqueAccountIDs = append(uniqueAccountIDs, entry)
		}
	}
	return uniqueAccountIDs
}

// ToAppRegionalResources takes an Application Resource Stack Instance stack, reads the output resources
// and returns a modeled  ProjectRegionalResources.
func ToAppRegionalResources(stack *cloudformation.Stack) (*AppRegionalResources, error) {
	regionalResources := AppRegionalResources{
		RepositoryURLs: map[string]string{},
	}
	for _, output := range stack.Outputs {
		key := *output.OutputKey
		value := *output.OutputValue

		switch {
		case key == appOutputKMSKey:
			regionalResources.KMSKeyARN = value
		case key == appOutputS3Bucket:
			regionalResources.S3Bucket = value
		case strings.HasPrefix(key, appOutputECRRepoPrefix):
			// If the output starts with the ECR Repo Prefix,
			// we'll pull the ARN out and construct a URL from it.
			uri, err := ecr.URIFromARN(value)
			if err != nil {
				return nil, err
			}
			// The service name for this repo is the Logical ID without
			// the ECR Repo prefix.
			safeSvcName := strings.TrimPrefix(key, appOutputECRRepoPrefix)
			// It's possible we had to sanitize the service name (removing dashes),
			// so return it back to its original form.
			originalSvcName := template.DashReplacedLogicalIDToOriginal(safeSvcName)
			regionalResources.RepositoryURLs[originalSvcName] = uri
		}
	}
	// Check to make sure the KMS key and S3 bucket exist in the stack. There isn't guranteed
	// to be any ECR repos (for a brand new env without any services), so we don't validate that.
	if regionalResources.KMSKeyARN == "" {
		return nil, fmt.Errorf("couldn't find KMS output key %s in stack %s", appOutputKMSKey, *stack.StackId)
	}

	if regionalResources.S3Bucket == "" {
		return nil, fmt.Errorf("couldn't find S3 bucket output key %s in stack %s", appOutputS3Bucket, *stack.StackId)
	}

	return &regionalResources, nil
}

// DNSDelegatedAccountsForStack looks through a stack's parameters for
// the parameter which stores the comma seperated list of account IDs
// which are permitted for DNS delegation.
func DNSDelegatedAccountsForStack(stack *cloudformation.Stack) []string {
	for _, parameter := range stack.Parameters {
		if *parameter.ParameterKey == appDNSDelegatedAccountsKey {
			return strings.Split(*parameter.ParameterValue, ",")
		}
	}

	return []string{}
}

func dnsDelegationRoleName(appName string) string {
	return fmt.Sprintf("%s-%s", appName, appDNSDelegationRoleName)
}
