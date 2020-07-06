package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

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

func TestTask_Template(t *testing.T) {
	testCases := map[string]struct {
		mockReadParser func(m *mocks.MockReadParser)

		wantedTemplate string
		wantedError    error
	}{
		"should return error if unable to read": {
			mockReadParser: func(m *mocks.MockReadParser) {
				m.EXPECT().Parse(taskTemplatePath, gomock.Any()).Return(nil, errors.New("error reading template"))
			},
			wantedError: errors.New("read template for task stack: error reading template"),
		},
		"should return template body when present": {
			mockReadParser: func(m *mocks.MockReadParser) {
				m.EXPECT().Parse(taskTemplatePath, gomock.Any()).Return(&template.Content{
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

			taskStack := &taskStackConfig{
				parser: mockReadParser,
			}

			got, err := taskStack.Template()

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Equal(t, tc.wantedTemplate, got)
			}
		})
	}
}

func TestTask_Parameters(t *testing.T) {
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(TaskNameParamKey),
			ParameterValue: aws.String("my-task"),
		},
		{
			ParameterKey:   aws.String(TaskContainerImageParamKey),
			ParameterValue: aws.String("7456.dkr.ecr.us-east-2.amazonaws.com/my-task:0.1"),
		},
		{
			ParameterKey:   aws.String(TaskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(TaskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(TaskLogRetentionParamKey),
			ParameterValue: aws.String(taskLogRetention),
		},
		{
			ParameterKey:   aws.String(TaskTaskRoleParamKey),
			ParameterValue: aws.String("task-role"),
		},
		{
			ParameterKey:   aws.String(TaskCommandParamKey),
			ParameterValue: aws.String("echo hooray"),
		},
	}

	task := &taskStackConfig{
		Name:   "my-task",
		Cpu:    256,
		Memory: 512,

		ImageURL: "7456.dkr.ecr.us-east-2.amazonaws.com/my-task:0.1",
		TaskRole: "task-role",
		Command:  "echo hooray",
	}
	params, _ := task.Parameters()
	require.ElementsMatch(t, expectedParams, params)
}

func TestTask_StackName(t *testing.T) {
	task := &taskStackConfig{
		Name: testTaskName,
	}
	got := task.StackName()
	require.Equal(t, got, fmt.Sprintf("task-%s", testTaskName))
}
