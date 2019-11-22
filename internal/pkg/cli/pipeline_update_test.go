// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	archermocks "github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUpdatePipelineOpts_convertStages(t *testing.T) {
	testCases := map[string]struct {
		stages        []manifest.PipelineStage
		inProjectName string

		mockWorkspace func(m *archermocks.MockWorkspace)
		mockEnvStore  func(m *archermocks.MockEnvironmentStore)

		expectedStages []deploy.PipelineStage
		expectedError  error
	}{
		"converts stages": {
			stages: []manifest.PipelineStage{
				{
					Name: "test",
				},
			},
			inProjectName: "badgoose",
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return([]archer.Manifest{
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "frontend",
						},
					},
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "backend",
						},
					}}, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				mockEnv := &archer.Environment{
					Name:      "test",
					Project:   "badgoose",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Prod:      false,
				}

				m.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1)
			},

			expectedStages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789012",
						Prod:      false,
					},
					LocalApplications: []string{"frontend", "backend"},
				},
			},
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEnvStore := archermocks.NewMockEnvironmentStore(ctrl)
			mockWorkspace := archermocks.NewMockWorkspace(ctrl)

			tc.mockEnvStore(mockEnvStore)
			tc.mockWorkspace(mockWorkspace)

			opts := &UpdatePipelineOpts{
				envStore: mockEnvStore,
				ws:       mockWorkspace,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
			}

			// WHEN
			actualStages, err := opts.convertStages(tc.stages)

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.expectedStages, actualStages)
			}
		})
	}

}

func TestInitPipelineOpts_getArtifactBuckets(t *testing.T) {
	testCases := map[string]struct {
		mockDeployer func(m *climocks.MockpipelineDeployer)

		expectedOut []deploy.ArtifactBucket

		expectedError error
	}{
		"getsBucketInfo": {
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				mockResources := []*archer.ProjectRegionalResources{
					{
						S3Bucket:  "someBucket",
						KMSKeyARN: "someKey",
					},
				}
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
			},
			expectedOut: []deploy.ArtifactBucket{
				{
					BucketName: "someBucket",
					KeyArn:     "someKey",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineDeployer := climocks.NewMockpipelineDeployer(ctrl)
			tc.mockDeployer(mockPipelineDeployer)

			opts := &UpdatePipelineOpts{
				pipelineDeployer: mockPipelineDeployer,
			}

			// WHEN
			actual, err := opts.getArtifactBuckets()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.expectedOut, actual)
			}
		})
	}
}
