// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvUpgradeOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		given     func(ctrl *gomock.Controller) *envUpgradeOpts
		wantedErr error
	}{
		"should not error if the environment exists and a name is provided": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockstore(ctrl)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
		},
		"should throw a config.ErrNoSuchEnvironment if the environment is not found": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockstore(ctrl)
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: "phonetool",
					EnvironmentName: "test",
				})

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
			wantedErr: &config.ErrNoSuchEnvironment{
				ApplicationName: "phonetool",
				EnvironmentName: "test",
			},
		},
		"should throw a wrapped error on unexpected config failure": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockstore(ctrl)
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
			wantedErr: errors.New("get environment test configuration from application phonetool: some error"),
		},
		"should not allow --all and --name": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
						all:     true,
					},
				}
			},
			wantedErr: errors.New("cannot specify both --all and --name flags"),
		},
		"should shorcircuit if only --all is provided": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						all:     true,
					},
				}
			},
			wantedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := tc.given(ctrl)

			err := opts.Validate()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnvUpgradeOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		given func(ctrl *gomock.Controller) *envUpgradeOpts

		wantedAppName string
		wantedEnvName string
		wantedErr     error
	}{
		"should prompt for application if not set": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockappEnvSelector(ctrl)
				m.EXPECT().Application("In which application is your environment?", "").Return("phonetool", nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						name: "test",
					},
					sel: m,
				}
			},
			wantedAppName: "phonetool",
			wantedEnvName: "test",
		},
		"should not prompt for environment if --all is set": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockappEnvSelector(ctrl)
				m.EXPECT().Application(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
						all:     true,
					},
					sel: m,
				}
			},
			wantedAppName: "phonetool",
			wantedEnvName: "test",
		},
		"should prompt for environment if --all and --name is not provided": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockappEnvSelector(ctrl)
				m.EXPECT().Environment(
					"Which environment do you want to upgrade?",
					`Upgrades the AWS CloudFormation template for your environment
to support the latest Copilot features.`,
					"phonetool").
					Return("test", nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
					},
					sel: m,
				}
			},
			wantedAppName: "phonetool",
			wantedEnvName: "test",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := tc.given(ctrl)

			err := opts.Ask()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedAppName, opts.appName)
				require.Equal(t, tc.wantedEnvName, opts.name)
			}
		})
	}
}

func TestEnvUpgradeOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		given     func(ctrl *gomock.Controller) *envUpgradeOpts
		wantedErr error
	}{
		"should skip upgrading if the environment version is already at least latest": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					{
						Name:   "test",
						Region: "us-west-2",
					},
					{
						Name:   "prod",
						Region: "us-east-1",
					},
				}, nil)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-east-1").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
				mockUploader := mocks.NewMockcustomResourcesUploader(ctrl)
				mockUploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil).Times(2)
				mockEnvTpl := mocks.NewMockversionGetter(ctrl)
				mockEnvTpl.EXPECT().Version().Return(deploy.LatestEnvTemplateVersion, nil).Times(2)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						all:     true,
					},
					store: mockStore,
					newEnvVersionGetter: func(_, _ string) (versionGetter, error) {
						return mockEnvTpl, nil
					},
					uploader: mockUploader,
					appCFN:   mockAppCFN,
					newS3: func(region string) (uploader, error) {
						return mocks.NewMockuploader(ctrl), nil
					},
				}
			},
		},
		"should upgrade non-legacy environments with UpgradeEnvironment call": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				mockEnvTpl := mocks.NewMockversionGetter(ctrl)
				mockEnvTpl.EXPECT().Version().Return("v0.1.0", nil) // Legacy versions are v0.0.0

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{
						App:              "phonetool",
						Name:             "test",
						Region:           "us-west-2",
						ExecutionRoleARN: "execARN",
						CustomConfig: &config.CustomizeEnv{
							ImportVPC: &config.ImportVPC{
								ID: "abc",
							},
						},
					}, nil)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket:  "mockBucket",
						KMSKeyARN: "mockKMS",
					}, nil)
				mockUploader := mocks.NewMockcustomResourcesUploader(ctrl)
				mockUploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(map[string]string{"mockCustomResource": "mockURL"}, nil)

				mockUpgrader := mocks.NewMockenvTemplateUpgrader(ctrl)
				mockUpgrader.EXPECT().UpgradeEnvironment(&deploy.CreateEnvironmentInput{
					Version: deploy.LatestEnvTemplateVersion,
					App: deploy.AppInformation{
						Name: "phonetool",
					},
					Name: "test",
					ImportVPCConfig: &config.ImportVPC{
						ID: "abc",
					},
					CFNServiceRoleARN:    "execARN",
					CustomResourcesURLs:  map[string]string{"mockCustomResource": "mockURL"},
					ArtifactBucketARN:    "arn:aws:s3:::mockBucket",
					ArtifactBucketKeyARN: "mockKMS",
				}).Return(nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: mockStore,
					prog:  mockProg,
					newEnvVersionGetter: func(_, _ string) (versionGetter, error) {
						return mockEnvTpl, nil
					},
					newTemplateUpgrader: func(conf *config.Environment) (envTemplateUpgrader, error) {
						return mockUpgrader, nil
					},
					uploader: mockUploader,
					appCFN:   mockAppCFN,
					newS3: func(region string) (uploader, error) {
						return mocks.NewMockuploader(ctrl), nil
					},
				}
			},
		},
		"should upgrade default legacy environments without any VPC configuration": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				mockEnvTpl := mocks.NewMockversionGetter(ctrl)
				mockEnvTpl.EXPECT().Version().Return(deploy.LegacyEnvTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{
						App:              "phonetool",
						Name:             "test",
						Region:           "us-west-2",
						ExecutionRoleARN: "execARN",
					}, nil)
				mockStore.EXPECT().ListServices("phonetool").Return([]*config.Workload{
					{
						App:  "phonetool",
						Name: "frontend",
						Type: manifest.LoadBalancedWebServiceType,
					},
					{
						App:  "phonetool",
						Name: "backend",
						Type: manifest.BackendServiceType,
					},
				}, nil)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket:  "mockBucket",
						KMSKeyARN: "mockKMS",
					}, nil)
				mockUploader := mocks.NewMockcustomResourcesUploader(ctrl)
				mockUploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(map[string]string{"mockCustomResource": "mockURL"}, nil)

				mockTemplater := mocks.NewMocktemplater(ctrl)
				mockTemplater.EXPECT().Template().Return("template", nil)

				mockUpgrader := mocks.NewMockenvTemplateUpgrader(ctrl)
				mockUpgrader.EXPECT().EnvironmentTemplate("phonetool", "test").Return("template", nil)
				mockUpgrader.EXPECT().UpgradeLegacyEnvironment(&deploy.CreateEnvironmentInput{
					Version: deploy.LatestEnvTemplateVersion,
					App: deploy.AppInformation{
						Name: "phonetool",
					},
					Name:                 "test",
					CFNServiceRoleARN:    "execARN",
					CustomResourcesURLs:  map[string]string{"mockCustomResource": "mockURL"},
					ArtifactBucketARN:    "arn:aws:s3:::mockBucket",
					ArtifactBucketKeyARN: "mockKMS",
				}, "frontend").Return(nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store:              mockStore,
					legacyEnvTemplater: mockTemplater,
					prog:               mockProg,
					newEnvVersionGetter: func(_, _ string) (versionGetter, error) {
						return mockEnvTpl, nil
					},
					newTemplateUpgrader: func(conf *config.Environment) (envTemplateUpgrader, error) {
						return mockUpgrader, nil
					},
					uploader: mockUploader,
					appCFN:   mockAppCFN,
					newS3: func(region string) (uploader, error) {
						return mocks.NewMockuploader(ctrl), nil
					},
				}
			},
		},
		"should upgrade legacy environments with imported VPC": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				mockEnvTpl := mocks.NewMockversionGetter(ctrl)
				mockEnvTpl.EXPECT().Version().Return(deploy.LegacyEnvTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{
						App:              "phonetool",
						Name:             "test",
						Region:           "us-west-2",
						ExecutionRoleARN: "execARN",
						CustomConfig: &config.CustomizeEnv{
							ImportVPC: &config.ImportVPC{
								ID: "abc",
							},
						},
					}, nil)
				mockStore.EXPECT().ListServices("phonetool").Return([]*config.Workload{}, nil)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
				mockUploader := mocks.NewMockcustomResourcesUploader(ctrl)
				mockUploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)

				mockTemplater := mocks.NewMocktemplater(ctrl)
				mockTemplater.EXPECT().Template().Return("template", nil)

				mockUpgrader := mocks.NewMockenvTemplateUpgrader(ctrl)
				mockUpgrader.EXPECT().EnvironmentTemplate("phonetool", "test").Return("modified template", nil)
				mockUpgrader.EXPECT().UpgradeLegacyEnvironment(&deploy.CreateEnvironmentInput{
					Version: deploy.LatestEnvTemplateVersion,
					App: deploy.AppInformation{
						Name: "phonetool",
					},
					Name:              "test",
					CFNServiceRoleARN: "execARN",
					ImportVPCConfig: &config.ImportVPC{
						ID: "abc",
					},
				}).Return(nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store:              mockStore,
					legacyEnvTemplater: mockTemplater,
					prog:               mockProg,
					newEnvVersionGetter: func(_, _ string) (versionGetter, error) {
						return mockEnvTpl, nil
					},
					newTemplateUpgrader: func(conf *config.Environment) (envTemplateUpgrader, error) {
						return mockUpgrader, nil
					},
					uploader: mockUploader,
					appCFN:   mockAppCFN,
					newS3: func(region string) (uploader, error) {
						return mocks.NewMockuploader(ctrl), nil
					},
				}
			},
		},
		"should throw an error if trying to upgrade a legacy environment with modified VPC but no SSM information": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				mockEnvTpl := mocks.NewMockversionGetter(ctrl)
				mockEnvTpl.EXPECT().Version().Return(deploy.LegacyEnvTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{
						App:              "phonetool",
						Name:             "test",
						Region:           "us-west-2",
						ExecutionRoleARN: "execARN",
					}, nil)
				mockStore.EXPECT().ListServices("phonetool").Return([]*config.Workload{}, nil)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
				mockAppCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
				mockUploader := mocks.NewMockcustomResourcesUploader(ctrl)
				mockUploader.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)

				mockTemplater := mocks.NewMocktemplater(ctrl)
				mockTemplater.EXPECT().Template().Return("template", nil)

				mockUpgrader := mocks.NewMockenvTemplateUpgrader(ctrl)
				mockUpgrader.EXPECT().EnvironmentTemplate(gomock.Any(), gomock.Any()).Return("modified template", nil)
				mockUpgrader.EXPECT().UpgradeLegacyEnvironment(gomock.Any()).Times(0)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store:              mockStore,
					legacyEnvTemplater: mockTemplater,
					prog:               mockProg,
					newEnvVersionGetter: func(_, _ string) (versionGetter, error) {
						return mockEnvTpl, nil
					},
					newTemplateUpgrader: func(conf *config.Environment) (envTemplateUpgrader, error) {
						return mockUpgrader, nil
					},
					uploader: mockUploader,
					appCFN:   mockAppCFN,
					newS3: func(region string) (uploader, error) {
						return mocks.NewMockuploader(ctrl), nil
					},
				}
			},
			wantedErr: errors.New("cannot upgrade environment due to missing vpc configuration"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := tc.given(ctrl)

			err := opts.Execute()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
