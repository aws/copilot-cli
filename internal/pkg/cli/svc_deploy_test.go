// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

func TestSvcDeployOpts_Validate(t *testing.T) {
	// There is no validation for svc deploy.
}

type svcDeployAskMocks struct {
	store *mocks.Mockstore
	sel   *mocks.MockwsSelector
	ws    *mocks.MockwsWlDirReader
}

func TestSvcDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inSvcName string

		setupMocks func(m *svcDeployAskMocks)

		wantedSvcName string
		wantedEnvName string
		wantedError   error
	}{
		"validate instead of prompting application name, svc name and environment name": {
			inAppName: "phonetool",
			inEnvName: "prod-iad",
			inSvcName: "frontend",
			setupMocks: func(m *svcDeployAskMocks) {
				m.store.EXPECT().GetApplication("phonetool")
				m.store.EXPECT().GetEnvironment("phonetool", "prod-iad").Return(&config.Environment{Name: "prod-iad"}, nil)
				m.ws.EXPECT().ListServices().Return([]string{"frontend"}, nil)
				m.store.EXPECT().GetService("phonetool", "frontend").Return(&config.Workload{}, nil)
				m.sel.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedSvcName: "frontend",
			wantedEnvName: "prod-iad",
		},
		"error instead of prompting for application name if not provided": {
			setupMocks: func(m *svcDeployAskMocks) {
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
			},
			wantedError: errNoAppInWorkspace,
		},
		"prompt for service name": {
			inAppName: "phonetool",
			inEnvName: "prod-iad",
			setupMocks: func(m *svcDeployAskMocks) {
				m.sel.EXPECT().Service("Select a service in your workspace", "").Return("frontend", nil)
				m.store.EXPECT().GetApplication(gomock.Any()).Times(1)
				m.store.EXPECT().GetEnvironment("phonetool", "prod-iad").Return(&config.Environment{Name: "prod-iad"}, nil)
				m.store.EXPECT().GetService("phonetool", "frontend").Return(&config.Workload{}, nil)
			},
			wantedSvcName: "frontend",
			wantedEnvName: "prod-iad",
		},
		"prompt for environment name": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			setupMocks: func(m *svcDeployAskMocks) {
				m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), "phonetool").Return("prod-iad", nil)
				m.store.EXPECT().GetApplication("phonetool")
				m.ws.EXPECT().ListServices().Return([]string{"frontend"}, nil)
				m.store.EXPECT().GetService("phonetool", "frontend").Return(&config.Workload{}, nil)
			},
			wantedSvcName: "frontend",
			wantedEnvName: "prod-iad",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &svcDeployAskMocks{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockwsSelector(ctrl),
				ws:    mocks.NewMockwsWlDirReader(ctrl),
			}
			tc.setupMocks(m)
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName: tc.inAppName,
					name:    tc.inSvcName,
					envName: tc.inEnvName,
				},
				sel:   m.sel,
				store: m.store,
				ws:    m.ws,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSvcName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

type deployMocks struct {
	mockDeployer             *mocks.MockworkloadDeployer
	mockInterpolator         *mocks.Mockinterpolator
	mockWsReader             *mocks.MockwsWlDirReader
	mockEnvFeaturesDescriber *mocks.MockversionCompatibilityChecker
	mockMft                  *mockWorkloadMft
	mockDiffWriter           *strings.Builder
	mockPrompter             *mocks.Mockprompter
	mockVersionGetter        *mocks.MockversionGetter
}

func TestSvcDeployOpts_Execute(t *testing.T) {
	const (
		mockAppName      = "phonetool"
		mockSvcName      = "frontend"
		mockEnvName      = "prod-iad"
		mockVersion      = "v1.29.0"
		mockNewerVersion = "v1.30.0"
	)
	mockError := errors.New("some error")
	mockErrStackNotFound := cloudformation.ErrStackNotFound{}
	testCases := map[string]struct {
		inShowDiff       bool
		inSkipDiffPrompt bool
		inForceFlag      bool
		inAllowDowngrade bool
		inSvcType        string
		mock             func(m *deployMocks)
		wantedDiff       string
		wantedError      error
	}{
		"error out if fail to get version": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return("", mockError)
			},

			wantedError: fmt.Errorf("get template version of workload frontend: some error"),
		},
		"error out if try to downgrade service version without flag": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockNewerVersion, nil)
			},

			wantedError: fmt.Errorf(`cannot downgrade workload "frontend" (currently in version v1.30.0) to version v1.29.0`),
		},
		"error out if fail to read workload manifest": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("read manifest file for frontend: some error"),
		},
		"error out if fail to interpolate workload manifest": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", mockError)
			},

			wantedError: fmt.Errorf("interpolate environment variables for frontend manifest: some error"),
		},
		"error if fail to get a list of available features from the environment": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get available features of the prod-iad environment stack: some error"),
		},
		"error out if force deploy for static site service": {
			inForceFlag: true,
			inSvcType:   manifestinfo.StaticSiteType,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
			},

			wantedError: fmt.Errorf(`--force is not supported for service type "Static Site"`),
		},
		"error if some required features are not available in the environment": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
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
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
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

			wantedError: fmt.Errorf("upload deploy resources for service frontend: some error"),
		},
		"error if failed to generate the template to show diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("generate the template for workload %q against environment %q: some error", mockSvcName, mockEnvName),
		},
		"error if failed to generate the diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"write 'no changes' if there is no diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "No changes.\n",
		},
		"write the correct diff": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "mock diff",
		},
		"error if fail to ask whether to continue the deployment": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			wantedError: errors.New("ask whether to continue with the deployment: some error"),
		},
		"do not deploy if asked to": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(false, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Times(0)
			},
		},
		"deploy if asked to": {
			inShowDiff: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Return(true, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Times(1)
			},
		},
		"skip prompt and deploy immediately after diff": {
			inShowDiff:       true,
			inSkipDiffPrompt: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&clideploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.mockDeployer.EXPECT().DeployDiff(gomock.Any()).Return("mock diff", nil)
				m.mockDiffWriter = &strings.Builder{}
				m.mockPrompter.EXPECT().Confirm(gomock.Eq("Continue with the deployment?"), gomock.Any(), gomock.Any()).Times(0)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Times(1)
			},
		},
		"error if failed to deploy service": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return(mockVersion, nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, mockError)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
			},

			wantedError: fmt.Errorf("deploy service frontend to environment prod-iad: some error"),
		},
		"success with no recommendations and allow downgrade": {
			inAllowDowngrade: true,
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Times(0)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
			},
		},
		"success for new deployment": {
			mock: func(m *deployMocks) {
				m.mockVersionGetter.EXPECT().Version().Return("", &mockErrStackNotFound)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&clideploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, nil)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
			},
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

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName:            mockAppName,
					name:               mockSvcName,
					envName:            mockEnvName,
					showDiff:           tc.inShowDiff,
					skipDiffPrompt:     tc.inSkipDiffPrompt,
					forceNewUpdate:     tc.inForceFlag,
					allowWkldDowngrade: tc.inAllowDowngrade,
					clientConfigured:   true,
				},
				svcType: tc.inSvcType,
				newSvcDeployer: func() (workloadDeployer, error) {
					return m.mockDeployer, nil
				},
				newInterpolator: func(app, env string) interpolator {
					return m.mockInterpolator
				},
				ws: m.mockWsReader,
				unmarshal: func(b []byte) (manifest.DynamicWorkload, error) {
					return m.mockMft, nil
				},
				envFeaturesDescriber: m.mockEnvFeaturesDescriber,
				prompt:               m.mockPrompter,
				diffWriter:           m.mockDiffWriter,
				svcVersionGetter:     m.mockVersionGetter,
				targetApp:            &config.Application{},
				targetEnv:            &config.Environment{},
				templateVersion:      mockVersion,
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

type checkEnvironmentCompatibilityMocks struct {
	ws                              *mocks.MockwsEnvironmentsLister
	versionFeatureGetter            *mocks.MockversionCompatibilityChecker
	requiredEnvironmentFeaturesFunc func() []string
}

func Test_isManifestCompatibleWithEnvironment(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMock    func(m *checkEnvironmentCompatibilityMocks)
		mockManifest *mockWorkloadMft
		wantedError  error
	}{
		"error getting environment version": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().Version().Return("", mockError)
			},
			wantedError: errors.New("get environment \"mockEnv\" version: some error"),
		},
		"error if env is not deployed": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().Version().Return(version.EnvTemplateBootstrap, nil)
			},
			wantedError: errors.New("cannot deploy a service to an undeployed environment. Please run \"copilot env deploy --name mockEnv\" to deploy the environment first"),
		},
		"error getting environment available features": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().Version().Return("mockVersion", nil)
				m.versionFeatureGetter.EXPECT().AvailableFeatures().Return(nil, mockError)
				m.requiredEnvironmentFeaturesFunc = func() []string {
					return nil
				}
			},
			wantedError: errors.New("get available features of the mockEnv environment stack: some error"),
		},
		"not compatible": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().Version().Return("mockVersion", nil)
				m.versionFeatureGetter.EXPECT().AvailableFeatures().Return([]string{template.ALBFeatureName}, nil)
				m.requiredEnvironmentFeaturesFunc = func() []string {
					return []string{template.InternalALBFeatureName}
				}
			},
			wantedError: errors.New(`environment "mockEnv" is on version "mockVersion" which does not support the "Internal ALB" feature`),
		},
		"compatible": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().Version().Return("mockVersion", nil)
				m.versionFeatureGetter.EXPECT().AvailableFeatures().Return([]string{template.ALBFeatureName, template.InternalALBFeatureName}, nil)
				m.requiredEnvironmentFeaturesFunc = func() []string {
					return []string{template.InternalALBFeatureName}
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &checkEnvironmentCompatibilityMocks{
				versionFeatureGetter: mocks.NewMockversionCompatibilityChecker(ctrl),
				ws:                   mocks.NewMockwsEnvironmentsLister(ctrl),
			}
			tc.setupMock(m)
			mockManifest := &mockWorkloadMft{
				mockRequiredEnvironmentFeatures: m.requiredEnvironmentFeaturesFunc,
			}

			// WHEN
			err := validateWorkloadManifestCompatibilityWithEnv(m.ws, m.versionFeatureGetter, mockManifest, "mockEnv")

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

type mockWorkloadMft struct {
	mockRequiredEnvironmentFeatures func() []string
}

func (m *mockWorkloadMft) ApplyEnv(envName string) (manifest.DynamicWorkload, error) {
	return m, nil
}

func (m *mockWorkloadMft) Validate() error {
	return nil
}

func (m *mockWorkloadMft) Load(sess *session.Session) error {
	return nil
}

func (m *mockWorkloadMft) Manifest() interface{} {
	return nil
}

func (m *mockWorkloadMft) RequiredEnvironmentFeatures() []string {
	return m.mockRequiredEnvironmentFeatures()
}
