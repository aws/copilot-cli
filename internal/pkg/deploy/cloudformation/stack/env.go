package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	*deploy.CreateEnvironmentInput
	parser template.ReadParser
}

const (
	// EnvTemplatePath is the path where the cloudformation for the environment is written.
	EnvTemplatePath           = "environment/cf.yml"
	acmValidationTemplatePath = "custom-resources/dns-cert-validator.js"
	dnsDelegationTemplatePath = "custom-resources/dns-delegation.js"
)

// Parameter keys.
const (
	envParamIncludeLBKey                = "IncludePublicLoadBalancer"
	envParamProjectNameKey              = "ProjectName"
	envParamEnvNameKey                  = "EnvironmentName"
	envParamToolsAccountPrincipalKey    = "ToolsAccountPrincipalARN"
	envParamProjectDNSKey               = "ProjectDNSName"
	envParamProjectDNSDelegationRoleKey = "ProjectDNSDelegationRole"
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

	content, err := e.parser.Parse(EnvTemplatePath, struct {
		DNSDelegationLambda string
		ACMValidationLambda string
	}{
		dnsLambda.String(),
		acmLambda.String(),
	})
	if err != nil {
		return "", err
	}

	return content.String(), nil
}

// Parameters returns the parameters to be passed into a environment CloudFormation template.
func (e *EnvStackConfig) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamIncludeLBKey),
			ParameterValue: aws.String(strconv.FormatBool(e.PublicLoadBalancer)),
		},
		{
			ParameterKey:   aws.String(envParamProjectNameKey),
			ParameterValue: aws.String(e.Project),
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
			ParameterKey:   aws.String(envParamProjectDNSKey),
			ParameterValue: aws.String(e.ProjectDNSName),
		},
		{
			ParameterKey:   aws.String(envParamProjectDNSDelegationRoleKey),
			ParameterValue: aws.String(e.dnsDelegationRole()),
		},
	}
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	tags := []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String(e.Project),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String(e.Name),
		},
	}
	for k, v := range e.AdditionalTags {
		tags = append(tags, &cloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return tags
}

func (e *EnvStackConfig) dnsDelegationRole() string {
	if e.ToolsAccountPrincipalARN == "" || e.ProjectDNSName == "" {
		return ""
	}

	projectRole, err := arn.Parse(e.ToolsAccountPrincipalARN)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", projectRole.AccountID, dnsDelegationRoleName(e.Project))
}

// StackName returns the name of the CloudFormation stack (based on the project and env names).
func (e *EnvStackConfig) StackName() string {
	return NameForEnv(e.Project, e.Name)
}

// ToEnv inspects an environment cloudformation stack and constructs an environment
// struct out of it (including resources like ECR Repo)
func (e *EnvStackConfig) ToEnv(stack *cloudformation.Stack) (*archer.Environment, error) {
	stackARN, err := arn.Parse(*stack.StackId)
	if err != nil {
		return nil, fmt.Errorf("couldn't extract region and account from stack ID %s: %w", *stack.StackId, err)
	}

	stackOutputs := make(map[string]string)
	for _, output := range stack.Outputs {
		stackOutputs[*output.OutputKey] = *output.OutputValue
	}

	return &archer.Environment{
		Name:             e.Name,
		Project:          e.Project,
		Prod:             e.Prod,
		Region:           stackARN.Region,
		AccountID:        stackARN.AccountID,
		ManagerRoleARN:   stackOutputs[EnvOutputManagerRoleKey],
		ExecutionRoleARN: stackOutputs[EnvOutputCFNExecutionRoleARN],
	}, nil
}
