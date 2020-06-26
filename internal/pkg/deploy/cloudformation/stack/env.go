package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	*deploy.CreateEnvironmentInput
	parser template.ReadParser
}

const (
	// EnvTemplatePath is the path where the cloudformation for the environment is written.
	EnvTemplatePath            = "environment/cf.yml"
	acmValidationTemplatePath  = "custom-resources/dns-cert-validator.js"
	dnsDelegationTemplatePath  = "custom-resources/dns-delegation.js"
	enableLongARNsTemplatePath = "custom-resources/enable-long-arns.js"
)

// Parameter keys.
const (
	envParamIncludeLBKey             = "IncludePublicLoadBalancer"
	envParamAppNameKey               = "AppName"
	envParamEnvNameKey               = "EnvironmentName"
	envParamToolsAccountPrincipalKey = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                = "AppDNSName"
	envParamAppDNSDelegationRoleKey  = "AppDNSDelegationRole"
)

// Output keys.
const (
	EnvOutputCFNExecutionRoleARN       = "CFNExecutionRoleARN"
	EnvOutputManagerRoleKey            = "EnvironmentManagerRoleARN"
	EnvOutputPublicLoadBalancerDNSName = "PublicLoadBalancerDNSName"
	EnvOutputSubdomain                 = "EnvironmentSubdomain"
)

// NewEnvStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up an environment.
func NewEnvStackConfig(input *deploy.CreateEnvironmentInput) *EnvStackConfig {
	return &EnvStackConfig{
		CreateEnvironmentInput: input,
		parser:                 template.New(),
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

	content, err := e.parser.Parse(EnvTemplatePath, struct {
		DNSDelegationLambda       string
		ACMValidationLambda       string
		EnableLongARNFormatLambda string
	}{
		dnsLambda.String(),
		acmLambda.String(),
		enableLongARNsLambda.String(),
	})
	if err != nil {
		return "", err
	}

	return content.String(), nil
}

// Parameters returns the parameters to be passed into a environment CloudFormation template.
func (e *EnvStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamIncludeLBKey),
			ParameterValue: aws.String(strconv.FormatBool(e.PublicLoadBalancer)),
		},
		{
			ParameterKey:   aws.String(envParamAppNameKey),
			ParameterValue: aws.String(e.AppName),
		},
		{
			ParameterKey:   aws.String(envParamEnvNameKey),
			ParameterValue: aws.String(e.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
			ParameterValue: aws.String(e.ToolsAccountPrincipalARN),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSKey),
			ParameterValue: aws.String(e.AppDNSName),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
			ParameterValue: aws.String(e.dnsDelegationRole()),
		},
	}, nil
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(e.AdditionalTags, map[string]string{
		AppTagKey: e.AppName,
		EnvTagKey: e.Name,
	})
}

func (e *EnvStackConfig) dnsDelegationRole() string {
	if e.ToolsAccountPrincipalARN == "" || e.AppDNSName == "" {
		return ""
	}

	appRole, err := arn.Parse(e.ToolsAccountPrincipalARN)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", appRole.AccountID, dnsDelegationRoleName(e.AppName))
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *EnvStackConfig) StackName() string {
	return NameForEnv(e.AppName, e.Name)
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
		Name:             e.Name,
		App:              e.AppName,
		Prod:             e.Prod,
		Region:           stackARN.Region,
		AccountID:        stackARN.AccountID,
		ManagerRoleARN:   stackOutputs[EnvOutputManagerRoleKey],
		ExecutionRoleARN: stackOutputs[EnvOutputCFNExecutionRoleARN],
	}, nil
}
