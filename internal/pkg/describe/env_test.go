// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	//"errors"
	//"fmt"
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
		testARN1         = "arn:aws:cloudformation:us-west-2:123456789012:stack/testProject-testEnv1-testApp1/6d75d1g0-8b1a-11ea-b358-06c1882c17fd"
		testARN2         = "arn:aws:cloudformation:us-west-2:123456789012:stack/testProject-testEnv1-testApp2/7d75d1f0-8c1a-11ea-b358-06c1882c17fc"
		testParsedStack1 = "testProject-test-app1"
		testParsedStack2 = "testProject-prod-app2"
		testParsedStack3 = "testProject-test-app3"
	)
	testProject := &archer.Project{
		Name: "testProject",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &archer.Environment{
		Project:          "testProject",
		Name:             "testEnv1",
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
	appList := []*archer.Application{testApp1, testApp2, testApp3}

	allApps := []*archer.Application{testApp1, testApp2, testApp3}
	env1Apps := []*archer.Application{testApp1, testApp2}
	//mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks envDescriberMocks)

		wantedEnv   *EnvDescription
		wantedApps  []*archer.Application
		wantedError error
	}{
		"success": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(

					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, map[string]string{stack.EnvTagKey: "testEnv1"}).Return([]string{
						testARN1,
						testARN2,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment:  testEnv,
				Applications: env1Apps,
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
				require.Equal(t, tc.wantedApps, actual)
			}
		})
	}
}
