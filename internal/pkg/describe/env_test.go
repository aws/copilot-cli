// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type envDescriberMocks struct {
	mockStoreService   *mocks.MockstoreSvc
	mockResourceGroupsClient *mocks.MockresourceGroupsClient
}

func TestEnvDescriber_FilterAppsForEnv(t *testing.T) {
	const (
		badParsedStack  = "badMockArn"
		testParsedStack1 = "phonetool-test-app1"
		testParsedStack2 = "phonetool-prod-app2"
	)
	testProject := &archer.Project{
		Name: "testProject",
	}
	testEnv1 := &archer.Environment{
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
		Name: "testApp1",
		Type: "load-balanced",
	}
	testApp3 :=	&archer.Application{
		Project: "testProject",
		Name:    "testApp3",
		Type: "load-balanced",
	}
	env1apps := []*archer.Application{testApp1, testApp3}
	badTags := map[string]string{stack.EnvTagKey: "badEnvName"}
	testCases := map[string]struct {
		env *archer.Environment
		proj *archer.Project
		apps []*archer.Application

		setupMocks func(mocks envDescriberMocks)
		mockrgClient	func(m *mocks.MockresourceGroupsClient)

		wantedApps   []*archer.Application
		wantedError error
	}{
		"return error if fail to get ARN": {
			env: testEnv1,
			proj: testProject,
			apps: env1apps,
			setupMocks: 	func(m envDescriberMocks){
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, badTags).Return())
			},


		},
		"return error if fail to get stack name from ARN": {

		},
		"successfully return correct apps for env": {

		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreService := mocks.NewMockstoreSvc(ctrl)
			mockResourceGroupsClient := mocks.NewMockresourceGroupsClient(ctrl)
			mocks := envDescriberMocks{
				mockStoreService:   mockStoreService,
				mockResourceGroupsClient: mockResourceGroupsClient,
			}

			tc.setupMocks(mocks)

			d := &EnvDescriber{
				env: testEnvironment1,
				proj: testProject,
				apps: env1apps,

				store: mockStoreService,
				rgClient: mockResourceGroupsClient,
			}

			// WHEN
			actual, err := d.FilterAppsForEnv()

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
