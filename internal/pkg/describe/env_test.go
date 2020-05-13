// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envDescriberMocks struct {
	storeSvc                 *mocks.MockstoreSvc
	mockResourceGroupsClient *mocks.MockresourceGroupsClient
}

func TestEnvDescriber_Describe(t *testing.T) {
	const (
		testARN1 = "arn:aws:cloudformation:us-west-2:123456789012:stack/testProject-testEnv-testApp1/6d75d1g0-8b1a-11ea-b358-06c1882c17fd"
		testARN2 = "arn:aws:cloudformation:us-west-2:123456789012:stack/testProject-testEnv-testApp2/7d75d1f0-8c1a-11ea-b358-06c1882c17fc"
		badARN   = "arn:aws:cloudformation:us-west-2:123456789012:stacktestProject-testEnv-testApp16d75d1g0-8b1a-11ea-b358-06c1882c17fd"
	)
	testProject := &archer.Project{
		Name: "testProject",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &archer.Environment{
		Project:          "testProject",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testApp1 := &archer.Application{
		Project: "testProject",
		Name:    "testApp1",
		Type:    "load-balanced",
	}
	testApp2 := &archer.Application{
		Project: "testProject",
		Name:    "testApp2",
		Type:    "load-balanced",
	}
	testApp3 := &archer.Application{
		Project: "testProject",
		Name:    "testApp3",
		Type:    "load-balanced",
	}

	allApps := []*archer.Application{testApp1, testApp2, testApp3}
	envApps := []*archer.Application{testApp1, testApp2}
	rgTags := map[string]string{stack.EnvTagKey: "testEnv"}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks envDescriberMocks)

		wantedEnv   *EnvDescription
		wantedApps  []*archer.Application
		wantedError error
	}{
		"error if GetResourcesByTags fails": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).
						Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("get resources for env testEnv: some error"),
		},
		"error if getStackName fails because cannot parse resourceARN": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).Return([]string{
						badARN,
					}, nil),
				)
			},
			wantedError: fmt.Errorf("get stack name from arn arn:aws:cloudformation:us-west-2:123456789012:stacktestProject-testEnv-testApp16d75d1g0-8b1a-11ea-b358-06c1882c17fd: cannot parse ARN resource stacktestProject-testEnv-testApp16d75d1g0-8b1a-11ea-b358-06c1882c17fd"),
		},
		"success": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).Return([]string{
						testARN1,
						testARN2,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment:  testEnv,
				Applications: envApps,
				Tags:         map[string]string{"key1": "value1", "key2": "value2"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstoreSvc(ctrl)
			mockResourceGroupsClient := mocks.NewMockresourceGroupsClient(ctrl)
			mocks := envDescriberMocks{
				storeSvc:                 mockStore,
				mockResourceGroupsClient: mockResourceGroupsClient,
			}

			tc.setupMocks(mocks)

			d := &EnvDescriber{
				env:  testEnv,
				proj: testProject,
				apps: allApps,

				store:    mockStore,
				rgClient: mockResourceGroupsClient,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedEnv, actual)
			}
		})
	}
}

func TestEnvDescriber_JSONString(t *testing.T) {
	mockApplications := []*archer.Application{
		{Project: "my-project",
			Name: "my-app",
			Type: "lb-web-app"},
		{Project: "my-project",
			Name: "copilot-app",
			Type: "lb-web-app"},
	}
	mockProject := &archer.Project{
		Name: "my-project",
		Tags: map[string]string{"tag1": "value1", "tag2": "value2"},
	}
	mockEnv := &archer.Environment{
		Project:          "my-project",
		Name:             "test",
		Region:           "us-west-2",
		AccountID:        "123456789",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}

	testCases := map[string]struct {
		shouldOutputJSON bool
		wantedError      error
		wantedContent    string
	}{
		"correctly shows json output": {
			shouldOutputJSON: true,
			wantedContent:    "{\"environment\":{\"project\":\"my-project\",\"name\":\"test\",\"region\":\"us-west-2\",\"accountID\":\"123456789\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"applications\":[{\"project\":\"my-project\",\"name\":\"my-app\",\"type\":\"lb-web-app\"},{\"project\":\"my-project\",\"name\":\"copilot-app\",\"type\":\"lb-web-app\"}],\"tags\":{\"tag1\":\"value1\",\"tag2\":\"value2\"}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &EnvDescriber{
				env:  mockEnv,
				proj: mockProject,
				apps: mockApplications,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, actual)
			}
		})
	}
}

func TestEnvDescriber_HumanString(t *testing.T) {
	mockApplications := []*archer.Application{
		{Project: "my-project",
			Name: "my-app",
			Type: "lb-web-app"},
		{Project: "my-project",
			Name: "copilot-app",
			Type: "lb-web-app"},
	}
	mockProject := &archer.Project{
		Name: "my-project",
		Tags: map[string]string{"tag1": "value1", "tag2": "value2"},
	}
	mockEnv := &archer.Environment{
		Project:          "my-project",
		Name:             "test",
		Region:           "us-west-2",
		AccountID:        "123456789",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}

	testCases := map[string]struct {
		shouldOutputJSON bool
		wantedError      error
		wantedContent    string
	}{
		"correctly shows human output": {
			shouldOutputJSON: false,
			wantedContent: `About

  Name              test
  Production        false
  Region            us-west-2
  Account ID        123456789

Applications

  Name              Type
  my-app            lb-web-app
  copilot-app       lb-web-app

Tags

  Key               Value
  tag1              value1
  tag2              value2
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &EnvDescriber{
				env:  mockEnv,
				proj: mockProject,
				apps: mockApplications,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, actual)
			}
		})
	}
}
