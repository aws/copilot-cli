// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
	acmValidationTemplatePath  = "custom-resources/dns-cert-validator.js"
	dnsDelegationTemplatePath  = "custom-resources/dns-delegation.js"
	enableLongARNsTemplatePath = "custom-resources/enable-long-arns.js"

	// Mandatory parameter keys.
	envParamAppNameKey               = "AppName"
	envParamEnvNameKey               = "EnvironmentName"
	envParamToolsAccountPrincipalKey = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                = "AppDNSName"
	envParamAppDNSDelegationRoleKey  = "AppDNSDelegationRole"

	// Output keys.
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
	dnsLambda, err := e.parser.Read(dnsDelegationTemplatePath)
	if err != nil {
		return "", err
	}
	acmLambda, err := e.parser.Read(acmValidationTemplatePath)
	if err != nil {
		return "", err
	}
	enableLongARNsLambda, err := e.parser.Read(enableLongARNsTemplatePath)
	if err != nil {
		return "", err
	}
	vpcConf := &config.AdjustVPC{
		CIDR:               DefaultVPCCIDR,
		PrivateSubnetCIDRs: strings.Split(DefaultPrivateSubnetCIDRs, ","),
		PublicSubnetCIDRs:  strings.Split(DefaultPublicSubnetCIDRs, ","),
	}

	if e.in.AdjustVPCConfig != nil {
		vpcConf = e.in.AdjustVPCConfig
	}

	content, err := e.parser.ParseEnv(&template.EnvOpts{
		ACMValidationLambda:       acmLambda.String(),
		DNSDelegationLambda:       dnsLambda.String(),
		EnableLongARNFormatLambda: enableLongARNsLambda.String(),
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
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", appRole.AccountID, dnsDelegationRoleName(e.in.AppName))
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
