// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testTaskName = "my-task"
)

func TestTaskStackConfig_Template(t *testing.T) {
	testCases := map[string]struct {
		mockReadParser func(m *mocks.MockReadParser)

		wantedTemplate string
		wantedError    error
	}{
		"should return error if unable to read": {
			mockReadParser: func(m *mocks.MockReadParser) {
				m.EXPECT().Parse(taskTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("error reading template"))
			},
			wantedError: errors.New("read template for task stack: error reading template"),
		},
		"should return template body when present": {
			mockReadParser: func(m *mocks.MockReadParser) {
				m.EXPECT().Parse(taskTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("This is the task template"),
				}, nil)
			},
			wantedTemplate: "This is the task template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockReadParser := mocks.NewMockReadParser(ctrl)
			if tc.mockReadParser != nil {
				tc.mockReadParser(mockReadParser)
			}

			taskInput := deploy.CreateTaskResourcesInput{}

			taskStackConfig := &taskStackConfig{
				CreateTaskResourcesInput: &taskInput,
				parser:                   mockReadParser,
			}

			got, err := taskStackConfig.Template()

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Equal(t, tc.wantedTemplate, got)
			}
		})
	}
}

func TestTaskStackConfig_Parameters(t *testing.T) {
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(taskNameParamKey),
			ParameterValue: aws.String("my-task"),
		},
		{
			ParameterKey:   aws.String(taskContainerImageParamKey),
			ParameterValue: aws.String("7456.dkr.ecr.us-east-2.amazonaws.com/my-task:0.1"),
		},
		{
			ParameterKey:   aws.String(taskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(taskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(taskLogRetentionParamKey),
			ParameterValue: aws.String(taskLogRetentionInDays),
		},
		{
			ParameterKey:   aws.String(taskTaskRoleParamKey),
			ParameterValue: aws.String("task-role"),
		},
		{
			ParameterKey:   aws.String(taskCommandParamKey),
			ParameterValue: aws.String("echo hooray"),
		},
		{
			ParameterKey:   aws.String(taskEntryPointParamKey),
			ParameterValue: aws.String("exec,some command"),
		},
		{
			ParameterKey:   aws.String(taskEnvFileARNParamKey),
			ParameterValue: aws.String("arn:aws:s3:::somebucket/manual/1638391936/env"),
		},
		{
			ParameterKey:   aws.String(taskOSParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(taskArchParamKey),
			ParameterValue: aws.String(""),
		},
	}

	taskInput := deploy.CreateTaskResourcesInput{
		Name:   "my-task",
		CPU:    256,
		Memory: 512,

		Image:      "7456.dkr.ecr.us-east-2.amazonaws.com/my-task:0.1",
		TaskRole:   "task-role",
		EnvFileARN: "arn:aws:s3:::somebucket/manual/1638391936/env",
		Command:    []string{"echo hooray"},
		EntryPoint: []string{"exec", "some command"},
	}

	task := &taskStackConfig{
		CreateTaskResourcesInput: &taskInput,
	}
	params, _ := task.Parameters()
	require.ElementsMatch(t, expectedParams, params)
}

func TestTaskStackConfig_StackName(t *testing.T) {
	taskInput := deploy.CreateTaskResourcesInput{
		Name: "my-task",
	}

	task := &taskStackConfig{
		CreateTaskResourcesInput: &taskInput,
	}
	got := task.StackName()
	require.Equal(t, got, taskStackPrefix+testTaskName)
}

func TestTaskStackConfig_Tags(t *testing.T) {
	testCases := map[string]struct {
		input deploy.CreateTaskResourcesInput

		expectedTags []*cloudformation.Tag
	}{
		"with app and env": {
			input: deploy.CreateTaskResourcesInput{
				Name: "my-task",

				App: "my-app",
				Env: "test",
			},

			expectedTags: []*cloudformation.Tag{
				{
					Key:   aws.String(deploy.TaskTagKey),
					Value: aws.String("my-task"),
				},
				{
					Key:   aws.String(deploy.AppTagKey),
					Value: aws.String("my-app"),
				},
				{
					Key:   aws.String(deploy.EnvTagKey),
					Value: aws.String("test"),
				},
			},
		},
		"input without app or env": {
			input: deploy.CreateTaskResourcesInput{
				Name: "my-task",

				Env: "",
			},

			expectedTags: []*cloudformation.Tag{
				{
					Key:   aws.String(deploy.TaskTagKey),
					Value: aws.String("my-task"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			taskStackConfig := &taskStackConfig{
				CreateTaskResourcesInput: &tc.input,
			}
			tags := taskStackConfig.Tags()

			require.ElementsMatch(t, tc.expectedTags, tags)
		})
	}
}
