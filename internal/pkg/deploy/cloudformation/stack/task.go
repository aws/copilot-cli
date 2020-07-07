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

	TaskNameParamKey         = "TaskName"
	TaskCPUParamKey          = "TaskCPU"
	TaskMemoryParamKey       = "TaskMemory"
	TaskLogRetentionParamKey = "LogRetention"

	TaskContainerImageParamKey = "ContainerImage"
	TaskTaskRoleParamKey       = "TaskRole"
	TaskCommandParamKey        = "Command"
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

func (t *taskStackConfig) StackName() string {
	return NameForTask(t.Name)
}

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

func (t *taskStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(TaskNameParamKey),
			ParameterValue: aws.String(t.Name),
		},
		{
			ParameterKey:   aws.String(TaskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(&t.Cpu))),
		},
		{
			ParameterKey:   aws.String(TaskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(&t.Memory))),
		},
		{
			ParameterKey:   aws.String(TaskLogRetentionParamKey),
			ParameterValue: aws.String(logRetention),
		},
		{
			ParameterKey:   aws.String(TaskContainerImageParamKey),
			ParameterValue: aws.String(t.ImageURL),
		},
		{
			ParameterKey:   aws.String(TaskTaskRoleParamKey),
			ParameterValue: aws.String(t.TaskRole),
		},
		{
			ParameterKey:   aws.String(TaskCommandParamKey),
			ParameterValue: aws.String(t.Command),
		},
	}, nil
}

func (t *taskStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(map[string]string{
		deploy.TaskTagKey: t.Name,
	}, nil)
}
