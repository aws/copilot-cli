// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type uploadArtifactsMock struct {
	uploader *mocks.MockcustomResourcesUploader
	appCFN   *mocks.MockappResourcesGetter
	s3       *mocks.Mockuploader
}

func TestEnvDeployer_UploadArtifacts(t *testing.T) {
	const (
		mockManagerRoleARN = "mockManagerRoleARN"
		mockEnvRegion      = "mockEnvRegion"
	)
	mockApp := &config.Application{}
	testCases := map[string]struct {
		setUpMocks  func(m *uploadArtifactsMock)
		wantedOut   map[string]string
		wantedError error
	}{
		"fail to get app resource by region": {
			setUpMocks: func(m *uploadArtifactsMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get app resources in region %s: some error", mockEnvRegion),
		},
		"fail to find S3 bucket in the region": {
			setUpMocks: func(m *uploadArtifactsMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{}, nil)
			},
			wantedError: fmt.Errorf("cannot find the S3 artifact bucket in region %s", mockEnvRegion),
		},
		"fail to upload artifacts": {
			setUpMocks: func(m *uploadArtifactsMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.uploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
			wantedError: errors.New("upload custom resources to bucket mockS3Bucket: some error"),
		},
		"success with URL returned": {
			setUpMocks: func(m *uploadArtifactsMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.uploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(map[string]string{
					"mockResource": "mockURL",
				}, nil)
			},
			wantedOut: map[string]string{
				"mockResource": "mockURL",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &uploadArtifactsMock{
				uploader: mocks.NewMockcustomResourcesUploader(ctrl),
				appCFN:   mocks.NewMockappResourcesGetter(ctrl),
				s3:       mocks.NewMockuploader(ctrl),
			}
			tc.setUpMocks(m)

			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					ManagerRoleARN: mockManagerRoleARN,
					Region:         mockEnvRegion,
				},
				uploader: m.uploader,
				appCFN:   m.appCFN,
				s3:       m.s3,
			}

			got, gotErr := d.UploadArtifacts()
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedOut, got)
			}
		})
	}
}

type deployEnvironmentMock struct {
	appCFN      *mocks.MockappResourcesGetter
	envDeployer *mocks.MockenvironmentDeployer
}

func TestEnvDeployer_DeployEnvironment(t *testing.T) {
	const (
		mockManagerRoleARN      = "mockManagerRoleARN"
		mockCFNExecutionRoleARN = "mockCFNExecutionRoleARN"
		mockEnvRegion           = "us-west-2"
		mockAppName             = "mockApp"
		mockEnvName             = "mockEnv"
	)
	mockApp := &config.Application{
		Name: mockAppName,
	}
	testCases := map[string]struct {
		setUpMocks  func(m *deployEnvironmentMock)
		wantedError error
	}{
		"fail to get app resources by region": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get app resources in region %s: some error", mockEnvRegion),
		},
		"fail to deploy environment": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"successful environment deployment": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployEnvironmentMock{
				appCFN:      mocks.NewMockappResourcesGetter(ctrl),
				envDeployer: mocks.NewMockenvironmentDeployer(ctrl),
			}
			tc.setUpMocks(m)
			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					Name:           mockEnvName,
					ManagerRoleARN: mockManagerRoleARN,
					Region:         mockEnvRegion,
				},
				appCFN:      m.appCFN,
				envDeployer: m.envDeployer,
			}

			mockIn := &deployEnvironmentInput{
				rootUserARN:  "mockRootUserARN",
				isProduction: false,
				customResourcesURLs: map[string]string{
					"mockResource": "mockURL",
				},
			}
			gotErr := d.DeployEnvironment(mockIn)
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
