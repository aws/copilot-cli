package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

const (
	taskTemplatePath = "task/cf.yml"

	taskNameParamKey         = "TaskName"
	taskCPUParamKey          = "TaskCPU"
	taskMemoryParamKey       = "TaskMemory"
	taskLogRetentionParamKey = "LogRetention"

	taskContainerImageParamKey = "ContainerImage"
	taskTaskRoleParamKey       = "TaskRole"
	taskCommandParamKey        = "Command"

	taskLogRetention = "1"
)

type taskStackConfig struct {
	Name string

	Cpu    int
	Memory int

	ImageURL string
	TaskRole string
	Command  string
	EnvVars map[string]string

	parser template.ReadParser
}

// NewTaskStackConfig sets up a struct that provides stack configurations for CloudFormation
// to deploy the task resources stack.
func NewTaskStackConfig(taskOpts *deploy.CreateTaskResourcesInput) *taskStackConfig {
	return &taskStackConfig{
		Name:   taskOpts.Name,
		Cpu:    taskOpts.Cpu,
		Memory: taskOpts.Memory,

		ImageURL: taskOpts.Image,
		TaskRole: taskOpts.TaskRole,
		Command:  taskOpts.Command,
		EnvVars: taskOpts.EnvVars,

		parser: template.New(),
	}
}

// StackName returns the name of the CloudFormation stack for the task.
func (t *taskStackConfig) StackName() string {
	return NameForTask(t.Name)
}

// Template returns the task CloudFormation template.
func (t *taskStackConfig) Template() (string, error) {
	content, err := t.parser.Parse(taskTemplatePath, struct{
		EnvVars map[string]string
	}{
		EnvVars: t.EnvVars,
	})
	if err != nil {
		return "", fmt.Errorf("read template for task stack: %w", err)
	}
	return content.String(), nil
}

// Parameters returns the parameter values to be passed to the task CloudFormation template.
func (t *taskStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(taskNameParamKey),
			ParameterValue: aws.String(t.Name),
		},
		{
			ParameterKey:   aws.String(taskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(&t.Cpu))),
		},
		{
			ParameterKey:   aws.String(taskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(&t.Memory))),
		},
		{
			ParameterKey:   aws.String(taskLogRetentionParamKey),
			ParameterValue: aws.String(taskLogRetention),
		},
		{
			ParameterKey:   aws.String(taskContainerImageParamKey),
			ParameterValue: aws.String(t.ImageURL),
		},
		{
			ParameterKey:   aws.String(taskTaskRoleParamKey),
			ParameterValue: aws.String(t.TaskRole),
		},
		{
			ParameterKey:   aws.String(taskCommandParamKey),
			ParameterValue: aws.String(t.Command),
		},
	}, nil
}

// Tags returns the tags that should be applied to the task CloudFormation.
func (t *taskStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(map[string]string{
		deploy.TaskTagKey: t.Name,
	}, nil)
}
