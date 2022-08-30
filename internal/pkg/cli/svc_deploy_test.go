// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
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
}

func TestSvcDeployOpts_Execute(t *testing.T) {
	const (
		mockAppName = "phonetool"
		mockSvcName = "frontend"
		mockEnvName = "prod-iad"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		mock func(m *deployMocks)

		wantedError error
	}{
		"error out if fail to read workload manifest": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("read manifest file for frontend: some error"),
		},
		"error out if fail to interpolate workload manifest": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", mockError)
			},

			wantedError: fmt.Errorf("interpolate environment variables for frontend manifest: some error"),
		},
		"error if fail to get a list of available features from the environment": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get available features of the prod-iad environment stack: some error"),
		},
		"error if some required features are not available in the environment": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1", "mockFeature3"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockEnvFeaturesDescriber.EXPECT().Version().Return("v1.mock", nil)
			},

			wantedError: fmt.Errorf(`environment "prod-iad" is on version "v1.mock" which does not support the "mockFeature3" feature`),
		},
		"error if failed to upload artifacts": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockEnvFeaturesDescriber.EXPECT().Version().Times(0)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("upload deploy resources for service frontend: some error"),
		},
		"error if failed to deploy service": {
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockMft = &mockWorkloadMft{
					mockRequiredEnvironmentFeatures: func() []string {
						return []string{"mockFeature1"}
					},
				}
				m.mockEnvFeaturesDescriber.EXPECT().AvailableFeatures().Return([]string{"mockFeature1", "mockFeature2"}, nil)
				m.mockEnvFeaturesDescriber.EXPECT().Version().Times(0)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, mockError)
				m.mockDeployer.EXPECT().IsServiceAvailableInRegion("").Return(false, nil)
			},

			wantedError: fmt.Errorf("deploy service frontend to environment prod-iad: some error"),
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
			}
			tc.mock(m)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName: mockAppName,
					name:    mockSvcName,
					envName: mockEnvName,

					clientConfigured: true,
				},
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
				targetApp:            &config.Application{},
				targetEnv:            &config.Environment{},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
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
	testCases := map[string]struct {
		setupMock    func(m *checkEnvironmentCompatibilityMocks)
		mockManifest *mockWorkloadMft
		wantedError  error
	}{
		"error getting environment available features": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().AvailableFeatures().Return(nil, errors.New("some error"))
				m.requiredEnvironmentFeaturesFunc = func() []string {
					return nil
				}
			},
			wantedError: errors.New("get available features of the mockEnv environment stack: some error"),
		},
		"not compatible": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
				m.versionFeatureGetter.EXPECT().AvailableFeatures().Return([]string{template.ALBFeatureName}, nil)
				m.versionFeatureGetter.EXPECT().Version().Return("mockVersion", nil)
				m.requiredEnvironmentFeaturesFunc = func() []string {
					return []string{template.InternalALBFeatureName}
				}
			},
			wantedError: errors.New(`environment "mockEnv" is on version "mockVersion" which does not support the "Internal ALB" feature`),
		},
		"compatible": {
			setupMock: func(m *checkEnvironmentCompatibilityMocks) {
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
