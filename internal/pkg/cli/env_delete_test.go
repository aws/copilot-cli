// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteEnvOpts_Validate(t *testing.T) {
	const (
		testProjName       = "phonetool"
		testEnvName        = "test"
		testRegion         = "us-west-2"
		testManagerRoleARN = "arn:aws:iam::1111:role/phonetool-test-EnvManagerRole"
	)
	var storeWithEnv = func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
		envStore := mocks.NewMockEnvironmentStore(ctrl)
		envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(&archer.Environment{
			Project:        testProjName,
			Name:           testEnvName,
			Region:         testRegion,
			ManagerRoleARN: testManagerRoleARN,
		}, nil)
		return envStore
	}

	testCases := map[string]struct {
		inProjectName string
		inEnv         string
		mockStore     func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore
		mockRG        func(ctrl *gomock.Controller) *climocks.MockResourceGroupsTaggingAPIAPI

		wantedError error
	}{
		"failed to retrieve environment from store": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				envStore := mocks.NewMockEnvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(nil, errors.New("some error"))
				return envStore
			},
			mockRG: func(ctrl *gomock.Controller) *climocks.MockResourceGroupsTaggingAPIAPI {
				return nil
			},
			wantedError: errors.New("get environment test metadata in project phonetool: some error"),
		},
		"failed to get resources with tags": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockResourceGroupsTaggingAPIAPI {
				rg := climocks.NewMockResourceGroupsTaggingAPIAPI(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))
				return rg
			},
			wantedError: errors.New("find application cloudformation stacks: some error"),
		},
		"environment has applications": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockResourceGroupsTaggingAPIAPI {
				rg := climocks.NewMockResourceGroupsTaggingAPIAPI(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{
						{
							Tags: []*resourcegroupstaggingapi.Tag{
								{
									Key:   aws.String(stack.AppTagKey),
									Value: aws.String("frontend"),
								},
								{
									Key:   aws.String(stack.AppTagKey),
									Value: aws.String("backend"),
								},
							},
						},
					},
				}, nil)
				return rg
			},
			wantedError: errors.New("applications: 'frontend, backend' still exist within the environment test"),
		},
		"success on empty environment": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockResourceGroupsTaggingAPIAPI {
				rg := climocks.NewMockResourceGroupsTaggingAPIAPI(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &DeleteEnvOpts{
				EnvName:        tc.inEnv,
				store:          tc.mockStore(ctrl),
				resourceGroups: tc.mockRG(ctrl),
				GlobalOpts:     &GlobalOpts{projectName: tc.inProjectName},
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeleteEnvOpts_Execute(t *testing.T) {

}
