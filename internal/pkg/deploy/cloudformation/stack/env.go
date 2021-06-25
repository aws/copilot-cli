// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type envReadParser interface {
	template.ReadParser
	ParseEnv(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error)
}

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	in     *deploy.CreateEnvironmentInput
	parser envReadParser
}

const (
	// Mandatory parameter keys.
	envParamAppNameKey               = "AppName"
	envParamEnvNameKey               = "EnvironmentName"
	envParamToolsAccountPrincipalKey = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                = "AppDNSName"
	envParamAppDNSDelegationRoleKey  = "AppDNSDelegationRole"
	EnvParamAliasesKey               = "Aliases"

	// Output keys.
	EnvOutputVPCID               = "VpcId"
	EnvOutputPublicSubnets       = "PublicSubnets"
	EnvOutputPrivateSubnets      = "PrivateSubnets"
	envOutputCFNExecutionRoleARN = "CFNExecutionRoleARN"
	envOutputManagerRoleKey      = "EnvironmentManagerRoleARN"

	// Default parameter values
	DefaultVPCCIDR            = "10.0.0.0/16"
	DefaultPublicSubnetCIDRs  = "10.0.0.0/24,10.0.1.0/24"
	DefaultPrivateSubnetCIDRs = "10.0.2.0/24,10.0.3.0/24"
)

// NewEnvStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up an environment.
func NewEnvStackConfig(input *deploy.CreateEnvironmentInput) *EnvStackConfig {
	return &EnvStackConfig{
		in:     input,
		parser: template.New(),
	}
}

// Template returns the environment CloudFormation template.
func (e *EnvStackConfig) Template() (string, error) {
	vpcConf := &config.AdjustVPC{
		CIDR:               DefaultVPCCIDR,
		PrivateSubnetCIDRs: strings.Split(DefaultPrivateSubnetCIDRs, ","),
		PublicSubnetCIDRs:  strings.Split(DefaultPublicSubnetCIDRs, ","),
	}
	bucket, dnsCertValidator, err := s3.ParseURL(e.in.CustomResourcesURLs[template.DNSCertValidatorFileName])
	if err != nil {
		return "", err
	}
	_, dnsDelegation, err := s3.ParseURL(e.in.CustomResourcesURLs[template.DNSDelegationFileName])
	if err != nil {
		return "", err
	}
	_, enableLongARN, err := s3.ParseURL(e.in.CustomResourcesURLs[template.EnableLongARNsFileName])
	if err != nil {
		return "", err
	}
	_, customDomain, err := s3.ParseURL(e.in.CustomResourcesURLs[template.CustomDomainFileName])
	if err != nil {
		return "", err
	}

	if e.in.AdjustVPCConfig != nil {
		vpcConf = e.in.AdjustVPCConfig
	}

	content, err := e.parser.ParseEnv(&template.EnvOpts{
		DNSCertValidatorLambda:    dnsCertValidator,
		DNSDelegationLambda:       dnsDelegation,
		EnableLongARNFormatLambda: enableLongARN,
		CustomDomainLambda:        customDomain,
		ScriptBucketName:          bucket,
		ImportVPC:                 e.in.ImportVPCConfig,
		VPCConfig:                 vpcConf,
		Version:                   e.in.Version,
	}, template.WithFuncs(map[string]interface{}{
		"inc": template.IncFunc,
	}))
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the parameters to be passed into a environment CloudFormation template.
func (e *EnvStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamAppNameKey),
			ParameterValue: aws.String(e.in.AppName),
		},
		{
			ParameterKey:   aws.String(envParamEnvNameKey),
			ParameterValue: aws.String(e.in.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
			ParameterValue: aws.String(e.in.ToolsAccountPrincipalARN),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSKey),
			ParameterValue: aws.String(e.in.AppDNSName),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
			ParameterValue: aws.String(e.dnsDelegationRole()),
		},
	}, nil
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(e.in.AdditionalTags, map[string]string{
		deploy.AppTagKey: e.in.AppName,
		deploy.EnvTagKey: e.in.Name,
	})
}

func (e *EnvStackConfig) dnsDelegationRole() string {
	if e.in.ToolsAccountPrincipalARN == "" || e.in.AppDNSName == "" {
		return ""
	}

	appRole, err := arn.Parse(e.in.ToolsAccountPrincipalARN)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("arn:%s:iam::%s:role/%s", appRole.Partition, appRole.AccountID, dnsDelegationRoleName(e.in.AppName))
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *EnvStackConfig) StackName() string {
	return NameForEnv(e.in.AppName, e.in.Name)
}

// ToEnv inspects an environment cloudformation stack and constructs an environment
// struct out of it (including resources like ECR Repo)
func (e *EnvStackConfig) ToEnv(stack *cloudformation.Stack) (*config.Environment, error) {
	stackARN, err := arn.Parse(*stack.StackId)
	if err != nil {
		return nil, fmt.Errorf("couldn't extract region and account from stack ID %s: %w", *stack.StackId, err)
	}

	stackOutputs := make(map[string]string)
	for _, output := range stack.Outputs {
		stackOutputs[*output.OutputKey] = *output.OutputValue
	}

	return &config.Environment{
		Name:             e.in.Name,
		App:              e.in.AppName,
		Prod:             e.in.Prod,
		Region:           stackARN.Region,
		AccountID:        stackARN.AccountID,
		ManagerRoleARN:   stackOutputs[envOutputManagerRoleKey],
		ExecutionRoleARN: stackOutputs[envOutputCFNExecutionRoleARN],
	}, nil
}
