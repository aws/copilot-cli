package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/gobuffalo/packd"
)

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	*deploy.CreateEnvironmentInput
	box packd.Box
}

const (
	// EnvParamIncludeLBKey is the CF Param Key Name for whether to include a LB
	EnvParamIncludeLBKey = "IncludePublicLoadBalancer"

	// EnvParamProjectNameKey is the CF Param Key Name for providing the project name
	EnvParamProjectNameKey = "ProjectName"

	// EnvParamEnvNameKey is the CF Param Key Name for providing the environment name
	EnvParamEnvNameKey = "EnvironmentName"

	// EnvTemplatePath is the path where the cloudformation for the environment is written
	EnvTemplatePath = "environment/cf.yml"

	// EnvOutputECRKey is the CF Output Key Name for the ECR Repo Name
	EnvOutputECRKey = "ECRRepositoryName"

	envParamToolsAccountPrincipal = "ToolsAccountPrincipalARN"

	ecrURLFormatString = "%s.dkr.ecr.%s.amazonaws.com/%s"
)

// newEnvStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up an environment.
func NewEnvStackConfig(input *deploy.CreateEnvironmentInput, box packd.Box) *EnvStackConfig {
	return &EnvStackConfig{
		CreateEnvironmentInput: input,
		box:                    box,
	}
}

// Template returns the environment CloudFormation template.
func (e *EnvStackConfig) Template() (string, error) {
	template, err := e.box.FindString(EnvTemplatePath)
	if err != nil {
		return "", &ErrTemplateNotFound{templateLocation: EnvTemplatePath, parentErr: err}
	}
	return template, nil
}

// Parameters returns the parameters to be passed into a environment CloudFormation template.
func (e *EnvStackConfig) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(EnvParamIncludeLBKey),
			ParameterValue: aws.String(strconv.FormatBool(e.PublicLoadBalancer)),
		},
		{
			ParameterKey:   aws.String(EnvParamProjectNameKey),
			ParameterValue: aws.String(e.Project),
		},
		{
			ParameterKey:   aws.String(EnvParamEnvNameKey),
			ParameterValue: aws.String(e.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipal),
			ParameterValue: aws.String(e.ToolsAccountPrincipalARN),
		},
	}
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(e.Project),
		},
		{
			Key:   aws.String(envTagKey),
			Value: aws.String(e.Name),
		},
	}
}

// StackName returns the name of the CloudFormation stack (based on the project and env names).
func (e *EnvStackConfig) StackName() string {
	return fmt.Sprintf("%s-%s", e.Project, e.Name)
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

	createdEnv := archer.Environment{
		Name:      e.Name,
		Project:   e.Project,
		Prod:      e.Prod,
		Region:    stackARN.Region,
		AccountID: stackARN.AccountID,
	}

	if stackOutputs[EnvOutputECRKey] != "" {
		createdEnv.RegistryURL = fmt.Sprintf(ecrURLFormatString,
			createdEnv.AccountID,
			createdEnv.Region,
			stackOutputs[EnvOutputECRKey])
	}

	return &createdEnv, nil
}
