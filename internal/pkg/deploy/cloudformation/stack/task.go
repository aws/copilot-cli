// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"
	"strings"

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
	taskEntryPointParamKey     = "EntryPoint"
	taskEnvFileARNParamKey     = "EnvFileARN"
	taskOSParamKey             = "OS"
	taskArchParamKey           = "Arch"

	// TaskOutputS3Bucket is the CFN stack output logical ID for a task's S3 bucket.
	TaskOutputS3Bucket = "S3Bucket"

	taskLogRetentionInDays = "1"
)

type taskStackConfig struct {
	*deploy.CreateTaskResourcesInput
	parser template.ReadParser
}

// NewTaskStackConfig sets up a struct that provides stack configurations for CloudFormation
// to deploy the task resources stack.
func NewTaskStackConfig(taskOpts *deploy.CreateTaskResourcesInput) *taskStackConfig {
	return &taskStackConfig{
		CreateTaskResourcesInput: taskOpts,
		parser:                   template.New(),
	}
}

// StackName returns the name of the CloudFormation stack for the task.
func (t *taskStackConfig) StackName() string {
	return string(NameForTask(t.Name))
}

var cfnFuntion = map[string]interface{}{
	"isARN":           template.IsARNFunc,
	"trimSlashPrefix": template.TrimSlashPrefix,
}

// Template returns the task CloudFormation template.
func (t *taskStackConfig) Template() (string, error) {
	content, err := t.parser.Parse(taskTemplatePath, struct {
		EnvVars               map[string]string
		SSMParamSecrets       map[string]string
		SecretsManagerSecrets map[string]string
		App                   string
		Env                   string
		ExecutionRole         string
		PermissionsBoundary   string
	}{
		EnvVars:               t.EnvVars,
		SSMParamSecrets:       t.SSMParamSecrets,
		SecretsManagerSecrets: t.SecretsManagerSecrets,
		App:                   t.App,
		Env:                   t.Env,
		ExecutionRole:         t.ExecutionRole,
		PermissionsBoundary:   t.PermissionsBoundary,
	}, template.WithFuncs(cfnFuntion))
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
			ParameterValue: aws.String(strconv.Itoa(t.CPU)),
		},
		{
			ParameterKey:   aws.String(taskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(t.Memory)),
		},
		{
			ParameterKey:   aws.String(taskLogRetentionParamKey),
			ParameterValue: aws.String(taskLogRetentionInDays),
		},
		{
			ParameterKey:   aws.String(taskContainerImageParamKey),
			ParameterValue: aws.String(t.Image),
		},
		{
			ParameterKey:   aws.String(taskTaskRoleParamKey),
			ParameterValue: aws.String(t.TaskRole),
		},
		{
			ParameterKey:   aws.String(taskCommandParamKey),
			ParameterValue: aws.String(strings.Join(t.Command, ",")),
		},
		{
			ParameterKey:   aws.String(taskEntryPointParamKey),
			ParameterValue: aws.String(strings.Join(t.EntryPoint, ",")),
		},
		{
			ParameterKey:   aws.String(taskEnvFileARNParamKey),
			ParameterValue: aws.String(t.EnvFileARN),
		},
		{
			ParameterKey:   aws.String(taskOSParamKey),
			ParameterValue: aws.String(t.OS),
		},
		{
			ParameterKey:   aws.String(taskArchParamKey),
			ParameterValue: aws.String(t.Arch),
		},
	}, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *taskStackConfig) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
}

// Tags returns the tags that should be applied to the task CloudFormation.
func (t *taskStackConfig) Tags() []*cloudformation.Tag {
	tags := map[string]string{
		deploy.TaskTagKey: t.Name,
	}

	if t.Env != "" {
		tags[deploy.AppTagKey] = t.App
		tags[deploy.EnvTagKey] = t.Env
	}

	return mergeAndFlattenTags(t.AdditionalTags, tags)
}
