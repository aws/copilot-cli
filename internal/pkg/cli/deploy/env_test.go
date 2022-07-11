// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type uploadArtifactsMock struct {
	appCFN *mocks.MockappResourcesGetter
	s3     *mocks.Mockuploader
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
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", fmt.Errorf("some error"))
			},
			wantedError: errors.New("upload custom resources to bucket mockS3Bucket"),
		},
		"success with URL returned": {
			setUpMocks: func(m *uploadArtifactsMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				crs, err := customresource.Env(fakeTemplateFS())
				require.NoError(t, err)

				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.FunctionName())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			wantedOut: map[string]string{
				"CertificateValidationFunction": "",
				"CustomDomainFunction":          "",
				"DNSDelegationFunction":         "",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &uploadArtifactsMock{
				appCFN: mocks.NewMockappResourcesGetter(ctrl),
				s3:     mocks.NewMockuploader(ctrl),
			}
			tc.setUpMocks(m)

			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					ManagerRoleARN: mockManagerRoleARN,
					Region:         mockEnvRegion,
				},
				appCFN:     m.appCFN,
				s3:         m.s3,
				templateFS: fakeTemplateFS(),
			}

			got, gotErr := d.UploadArtifacts()
			if tc.wantedError != nil {
				require.Contains(t, gotErr.Error(), tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedOut, got)
			}
		})
	}
}

type deployEnvironmentMock struct {
	appCFN           *mocks.MockappResourcesGetter
	envDeployer      *mocks.MockenvironmentDeployer
	prefixListGetter *mocks.MockprefixListGetter
}

func TestEnvDeployer_DeployEnvironment(t *testing.T) {
	const (
		mockManagerRoleARN = "mockManagerRoleARN"
		mockEnvRegion      = "us-west-2"
		mockAppName        = "mockApp"
		mockEnvName        = "mockEnv"
	)
	mockApp := &config.Application{
		Name: mockAppName,
	}
	testCases := map[string]struct {
		setUpMocks  func(m *deployEnvironmentMock)
		inManifest  *manifest.Environment
		wantedError error
	}{
		"fail to get app resources by region": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get app resources in region %s: some error", mockEnvRegion),
		},
		"fail to get prefix list id": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return(nil, errors.New("some error"))
			},
			inManifest: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					HTTPConfig: manifest.EnvironmentHTTPConfig{
						Public: manifest.PublicHTTPConfig{
							Ingress: manifest.Ingress{
								CDNIngress: aws.Bool(true),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf("retrieve CloudFront managed prefix list id: some error"),
		},
		"prefix list not retrieved when manifest not present": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return(aws.String("mockPrefixListID"), nil).Times(0)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			inManifest: nil,
		},
		"fail to deploy environment": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return(aws.String("mockPrefixListID"), nil).Times(0)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"successful environment deployment": {
			setUpMocks: func(m *deployEnvironmentMock) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&stack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return(aws.String("mockPrefixListID"), nil).Times(0)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ progress.FileWriter, in *deploy.CreateEnvironmentInput, opts ...cloudformation.StackOption) error {
						require.Equal(t, mockEnvName, in.Name)
						require.Equal(t, mockAppName, in.App.Name)
						require.Equal(t, map[string]string{
							"mockResource": "mockURL",
						}, in.CustomResourcesURLs)
						require.Equal(t, deploy.LatestEnvTemplateVersion, in.Version)
						return nil
					})
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployEnvironmentMock{
				appCFN:           mocks.NewMockappResourcesGetter(ctrl),
				envDeployer:      mocks.NewMockenvironmentDeployer(ctrl),
				prefixListGetter: mocks.NewMockprefixListGetter(ctrl),
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
				ec2:         m.prefixListGetter,
			}
			mockIn := &DeployEnvironmentInput{
				RootUserARN: "mockRootUserARN",
				CustomResourcesURLs: map[string]string{
					"mockResource": "mockURL",
				},
				Manifest: tc.inManifest,
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
