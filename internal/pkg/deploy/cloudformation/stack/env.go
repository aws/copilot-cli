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
	// Parameter keys.
	envParamAppNameKey               = "AppName"
	envParamEnvNameKey               = "EnvironmentName"
	envParamToolsAccountPrincipalKey = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                = "AppDNSName"
	envParamAppDNSDelegationRoleKey  = "AppDNSDelegationRole"
	EnvParamAliasesKey               = "Aliases"
	EnvParamALBWorkloadsKey          = "ALBWorkloads"
	envParamEFSWorkloadsKey          = "EFSWorkloads"
	envParamNATWorkloadsKey          = "NATWorkloads"
	envParamCreateHTTPSListenerKey   = "CreateHTTPSListener"
	EnvParamServiceDiscoveryEndpoint = "ServiceDiscoveryEndpoint"

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

var (
	fmtServiceDiscoveryEndpoint = "%s.%s.local"
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
	_, customDomain, err := s3.ParseURL(e.in.CustomResourcesURLs[template.CustomDomainFileName])
	if err != nil {
		return "", err
	}

	if e.in.AdjustVPCConfig != nil {
		vpcConf = e.in.AdjustVPCConfig
	}

	content, err := e.parser.ParseEnv(&template.EnvOpts{
		AppName:                e.in.App.Name,
		DNSCertValidatorLambda: dnsCertValidator,
		DNSDelegationLambda:    dnsDelegation,
		CustomDomainLambda:     customDomain,
		ScriptBucketName:       bucket,
		ArtifactBucketARN:      e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN:   e.in.ArtifactBucketKeyARN,
		ImportVPC:              e.in.ImportVPCConfig,
		ImportCertARNs:         e.in.ImportCertARNs,
		VPCConfig:              vpcConf,
		Version:                e.in.Version,
		Telemetry:              e.in.Telemetry,
		LatestVersion:          deploy.LatestEnvTemplateVersion,
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
	httpsListener := "false"
	if len(e.in.ImportCertARNs) != 0 || e.in.App.Domain != "" {
		httpsListener = "true"
	}
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamAppNameKey),
			ParameterValue: aws.String(e.in.App.Name),
		},
		{
			ParameterKey:   aws.String(envParamEnvNameKey),
			ParameterValue: aws.String(e.in.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
			ParameterValue: aws.String(e.in.App.AccountPrincipalARN),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSKey),
			ParameterValue: aws.String(e.in.App.Domain),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
			ParameterValue: aws.String(e.in.App.DNSDelegationRole()),
		},
		{
			ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
			ParameterValue: aws.String(fmt.Sprintf(fmtServiceDiscoveryEndpoint, e.in.Name, e.in.App.Name)),
		},
		{
			ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
			ParameterValue: aws.String(httpsListener),
		},
		{
			ParameterKey:   aws.String(EnvParamAliasesKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(envParamEFSWorkloadsKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(envParamNATWorkloadsKey),
			ParameterValue: aws.String(""),
		},
	}, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *EnvStackConfig) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(e.in.AdditionalTags, map[string]string{
		deploy.AppTagKey: e.in.App.Name,
		deploy.EnvTagKey: e.in.Name,
	})
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *EnvStackConfig) StackName() string {
	return NameForEnv(e.in.App.Name, e.in.Name)
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
		App:              e.in.App.Name,
		Prod:             e.in.Prod,
		Region:           stackARN.Region,
		AccountID:        stackARN.AccountID,
		ManagerRoleARN:   stackOutputs[envOutputManagerRoleKey],
		ExecutionRoleARN: stackOutputs[envOutputCFNExecutionRoleARN],
	}, nil
}
