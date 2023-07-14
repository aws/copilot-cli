// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestJobDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inJobName string

		mockWs    func(m *mocks.MockwsWlDirReader)
		mockStore func(m *mocks.Mockstore)

		wantedError error
	}{
		"no existing applications": {
			mockWs:    func(m *mocks.MockwsWlDirReader) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errNoAppInWorkspace,
		},
		"with workspace error": {
			inAppName: "phonetool",
			inJobName: "resizer",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListJobs().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("list jobs in the workspace: some error"),
		},
		"with job not in workspace": {
			inAppName: "phonetool",
			inJobName: "resizer",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListJobs().Return([]string{}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("job resizer not found in the workspace"),
		},
		"with unknown environment": {
			inAppName: "phonetool",
			inEnvName: "test",
			mockWs:    func(m *mocks.MockwsWlDirReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(nil, errors.New("unknown env"))
			},

			wantedError: errors.New("get environment test configuration: unknown env"),
		},
		"successful validation": {
			inAppName: "phonetool",
			inJobName: "resizer",
			inEnvName: "test",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListJobs().Return([]string{"resizer"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWs := mocks.NewMockwsWlDirReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					appName: tc.inAppName,
					name:    tc.inJobName,
					envName: tc.inEnvName,
				},
				ws:    mockWs,
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJobDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inEnvName  string
		inJobName  string
		inImageTag string

		wantedCalls func(m *mocks.MockwsSelector)

		wantedJobName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"prompts for environment name and job names": {
			inAppName:  "phonetool",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job("Select a job from your workspace", "").Return("resizer", nil)
				m.EXPECT().Environment("Select an environment", "", "phonetool").Return("prod-iad", nil)
			},

			wantedJobName:  "resizer",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
		"don't call selector if flags are provided": {
			inAppName:  "phonetool",
			inEnvName:  "prod-iad",
			inJobName:  "resizer",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedJobName:  "resizer",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockSel := mocks.NewMockwsSelector(ctrl)

			tc.wantedCalls(mockSel)
			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					appName:  tc.inAppName,
					name:     tc.inJobName,
					envName:  tc.inEnvName,
					imageTag: tc.inImageTag,
				},
				sel: mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedJobName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
				require.Equal(t, tc.wantedImageTag, opts.imageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestJobDeployOpts_Execute(t *testing.T) {
	const (
		mockAppName         = "phonetool"
		mockJobName         = "upload"
		mockEnvName         = "prod-iad"
		mockTemplateVersion = "v1.28.0"
		mockPrevVersion     = "v1.27.0"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		inShowDiff       bool
		inAllowDowngrade bool
		mock             func(m *deployMocks)

		wantedDiff  string
		wantedError error
	}{
		"error out if fail to get version": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return("", mockError)
			},

			wantedError: fmt.Errorf("get template version of workload upload: some error"),
		},
		"error out if fail to read workload manifest": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockPrevVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("read manifest file for upload: some error"),
		},
		"error out if fail to interpolate workload manifest": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", mockError)
			},

			wantedError: fmt.Errorf("interpolate environment variables for upload manifest: some error"),
		},
		"error if fail to get a list of available features from the environment": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get available features of the prod-iad environment stack: some error"),
		},
		"error if some required features are not available in the environment": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1", "mockFeature3"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
			},
			wantedError: fmt.Errorf(`environment "prod-iad" is on version "v1.mock" which does not support the "mockFeature3" feature`),
		},
		"error if failed to upload artifacts": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("upload deploy resources for job upload: some error"),
		},
		"error if failed to generate the template to show diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("generate the template for job %q against environment %q: some error", mockJobName, mockEnvName),
		},
		"error if failed to generate the diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"write 'no changes' if there is no diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "No changes.\n",
		},
		"write the correct diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "mock diff",
		},
		"error if fail to ask whether to continue the deployment": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			wantedError: errors.New("ask whether to continue with the deployment: some error"),
		},
		"do not deploy if asked to": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Times(0)
			},
		},
		"deploy if asked to and allow downgrade": {
			inShowDiff:       true,
			inAllowDowngrade: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(true, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Times(1)
			},
		},
		"error if failed to deploy service": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockJobName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, mockError)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
			},

			wantedError: fmt.Errorf("deploy job upload to environment prod-iad: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockDeployer:             mocks.NewMockworkloadDeployer(ctrl),
				mockInterpolator:         mocks.NewMockinterpolator(ctrl),
				mockWsReader:             mocks.NewMockwsWlDirReader(ctrl),
				mockEnvFeaturesDescriber: mocks.NewMockversionCompatibilityChecker(ctrl),
				mockPrompter:             mocks.NewMockprompter(ctrl),
				mockVersionGetter:        mocks.NewMockversionGetter(ctrl),
			}
			tc.mock(m)

			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					appName:            mockAppName,
					name:               mockJobName,
					envName:            mockEnvName,
					showDiff:           tc.inShowDiff,
					allowWkldDowngrade: tc.inAllowDowngrade,

					clientConfigured: true,
				},
				ws: m.mockWsReader,
				newJobDeployer: func() (workloadDeployer, error) {
					return m.mockDeployer, nil
				},
				newInterpolator: func(app, env string) interpolator {
					return m.mockInterpolator
				},
				unmarshal: func(b []byte) (manifest.DynamicWorkload, error) {
					return m.mockMft, nil
				},
				jobVersionGetter:     m.mockVersionGetter,
				envFeaturesDescriber: m.mockEnvFeaturesDescriber,
				prompt:               m.mockPrompter,
				diffWriter:           m.mockDiffWriter,
				templateVersion:      mockTemplateVersion,

				targetApp: &config.Application{},
				targetEnv: &config.Environment{},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
			if tc.wantedDiff != "" {
				require.Equal(t, tc.wantedDiff, m.mockDiffWriter.String())
			}
		})
	}
}
