// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
)

func TestCloudFormation_DeployTask(t *testing.T) {
	mockTask := &deploy.CreateTaskResourcesInput{
		Name: "my-task",
	}

	testCases := map[string]struct {
		mockCfnClient func(m *mocks.MockcfnClient)
		wantedError   error
	}{
		"create a new stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
			},
		},
		"failed to create stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("error"))
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
			},
			wantedError: errors.New("create stack: error"),
		},
		"update the stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
					Name: "my-task",
				})
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(1).Return(nil)
			},
		},
		"failed to update stack": {
			mockCfnClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
					Name: "my-task",
				})
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(errors.New("error"))
			},
			wantedError: errors.New("update stack: error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCfnClient := mocks.NewMockcfnClient(ctrl)
			if tc.mockCfnClient != nil {
				tc.mockCfnClient(mockCfnClient)
			}

			cf := CloudFormation{
				cfnClient: mockCfnClient,
			}

			err := cf.DeployTask(mockTask)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

var mockDescription1 = &cloudformation.StackDescription{
	Tags: []*awscfn.Tag{
		{
			Key: aws.String("copilot-task"),
		},
		{
			Key:   aws.String("copilot-application"),
			Value: aws.String("appname"),
		},
		{
			Key:   aws.String("copilot-environment"),
			Value: aws.String("test"),
		},
	},
	StackName: aws.String("task-database"),
	RoleARN:   aws.String("arn:aws:iam::123456789012:role/appname-test-CFNExecutionRole"),
}
var mockDescription2 = &cloudformation.StackDescription{
	Tags: []*awscfn.Tag{
		{
			Key: aws.String("copilot-task"),
		},
		{
			Key:   aws.String("copilot-application"),
			Value: aws.String("otherapp"),
		},
		{
			Key:   aws.String("copilot-environment"),
			Value: aws.String("test"),
		},
	},
	StackName: aws.String("task-example"),
	RoleARN:   aws.String("arn:aws:iam::123456789012:role/otherapp-staging-CFNExecutionRole"),
}

var mockDescription3 = &cloudformation.StackDescription{
	Tags: []*awscfn.Tag{
		{
			Key: aws.String("copilot-task"),
		},
	},
	StackName: aws.String("task-default"),
	RoleARN:   aws.String(""),
}

func TestCloudFormation_ListTaskStacks(t *testing.T) {
	testCases := map[string]struct {
		inAppName   string
		mockClient  func(*mocks.MockcfnClient)
		wantedErr   string
		wantedTasks []deploy.TaskStackInfo
	}{
		"successfully gets task stacks while excluding wrongly tagged stack": {
			inAppName: "appname",
			mockClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().ListStacksWithTags(map[string]string{
					"copilot-application": "appname",
					"copilot-environment": "test",
					"copilot-task":        "",
				}).Return([]cloudformation.StackDescription{
					*mockDescription1,
				}, nil)
			},
			wantedTasks: []deploy.TaskStackInfo{
				{
					StackName: "task-database",
					App:       "appname",
					Env:       "test",
					RoleARN:   aws.StringValue(mockDescription1.RoleARN),
				},
			},
		},
		"error listing stacks": {
			inAppName: "appname",
			mockClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().ListStacksWithTags(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: "some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCf := mocks.NewMockcfnClient(ctrl)
			tc.mockClient(mockCf)

			cf := CloudFormation{cfnClient: mockCf}

			// WHEN
			tasks, err := cf.ListTaskStacks("appname", "test")

			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, tc.wantedTasks, tasks)
			}
		})
	}
}

func TestCloudFormation_GetTaskDefaultStackInfo(t *testing.T) {
	testCases := map[string]struct {
		inAppName   string
		mockClient  func(*mocks.MockcfnClient)
		wantedErr   string
		wantedTasks []deploy.TaskStackInfo
	}{
		"successfully gets task stacks while excluding wrongly tagged stack": {
			inAppName: "appname",
			mockClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().ListStacksWithTags(map[string]string{
					"copilot-task": "",
				}).Return([]cloudformation.StackDescription{
					*mockDescription1,
					*mockDescription2,
					*mockDescription3,
				}, nil)
			},
			wantedTasks: []deploy.TaskStackInfo{
				{
					StackName: "task-default",
				},
			},
		},

		"error listing stacks": {
			inAppName: "appname",
			mockClient: func(m *mocks.MockcfnClient) {
				m.EXPECT().ListStacksWithTags(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: "some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCf := mocks.NewMockcfnClient(ctrl)
			tc.mockClient(mockCf)

			cf := CloudFormation{cfnClient: mockCf}

			// WHEN
			tasks, err := cf.ListDefaultTaskStacks()

			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, tc.wantedTasks, tasks)
			}

		})
	}

}
