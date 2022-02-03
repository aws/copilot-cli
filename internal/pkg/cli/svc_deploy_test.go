// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

func TestSvcDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inSvcName string

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
			inSvcName: "frontend",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListServices().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("list services in the workspace: some error"),
		},
		"with service not in workspace": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListServices().Return([]string{}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("service frontend not found in the workspace"),
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
			inSvcName: "frontend",
			inEnvName: "test",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListServices().Return([]string{"frontend"}, nil)
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
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName: tc.inAppName,
					name:    tc.inSvcName,
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

func TestSvcDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inEnvName  string
		inSvcName  string
		inImageTag string

		wantedCalls func(m *mocks.MockwsSelector)

		wantedSvcName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"prompts for environment name and service names": {
			inAppName:  "phonetool",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service("Select a service in your workspace", "").Return("frontend", nil)
				m.EXPECT().Environment("Select an environment", "", "phonetool").Return("prod-iad", nil)
			},

			wantedSvcName:  "frontend",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
		"don't call selector if flags are provided": {
			inAppName:  "phonetool",
			inEnvName:  "prod-iad",
			inSvcName:  "frontend",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedSvcName:  "frontend",
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
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName:  tc.inAppName,
					name:     tc.inSvcName,
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
				require.Equal(t, tc.wantedSvcName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
				require.Equal(t, tc.wantedImageTag, opts.imageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

type deployMocks struct {
	mockDeployer     *mocks.MockworkloadDeployer
	mockEnvUpgrader  *mocks.MockactionCommand
	mockInterpolator *mocks.Mockinterpolator
	mockWsReader     *mocks.MockwsWlDirReader
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
		"error if failed to upgrade environment": {
			mock: func(m *deployMocks) {
				m.mockEnvUpgrader.EXPECT().Execute().Return(mockError)
			},

			wantedError: fmt.Errorf(`execute "env upgrade --app phonetool --name prod-iad": some error`),
		},
		"error out if fail to read workload manifest": {
			mock: func(m *deployMocks) {
				m.mockEnvUpgrader.EXPECT().Execute().Return(nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("read manifest file for frontend: some error"),
		},
		"error out if fail to interpolate workload manifest": {
			mock: func(m *deployMocks) {
				m.mockEnvUpgrader.EXPECT().Execute().Return(nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", mockError)
			},

			wantedError: fmt.Errorf("interpolate environment variables for frontend manifest: some error"),
		},
		"error if failed to upload artifacts": {
			mock: func(m *deployMocks) {
				m.mockEnvUpgrader.EXPECT().Execute().Return(nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("upload deploy resources for service frontend: some error"),
		},
		"error if failed to deploy service": {
			mock: func(m *deployMocks) {
				m.mockEnvUpgrader.EXPECT().Execute().Return(nil)
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockSvcName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockDeployer.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{}, nil)
				m.mockDeployer.EXPECT().DeployWorkload(gomock.Any()).Return(nil, mockError)
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
				mockDeployer:     mocks.NewMockworkloadDeployer(ctrl),
				mockEnvUpgrader:  mocks.NewMockactionCommand(ctrl),
				mockInterpolator: mocks.NewMockinterpolator(ctrl),
				mockWsReader:     mocks.NewMockwsWlDirReader(ctrl),
			}
			tc.mock(m)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName: mockAppName,
					name:    mockSvcName,
					envName: mockEnvName,

					clientConfigured: true,
				},
				newSvcDeployer: func(dso *deploySvcOpts) (workloadDeployer, error) {
					return m.mockDeployer, nil
				},
				envUpgradeCmd: m.mockEnvUpgrader,
				newInterpolator: func(app, env string) interpolator {
					return m.mockInterpolator
				},
				ws: m.mockWsReader,
				unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &mockWorkloadMft{}, nil
				},
				uploadOpts: &uploadCustomResourcesOpts{},

				targetApp:    &config.Application{},
				appResources: &stack.AppRegionalResources{},
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

type mockWorkloadMft struct{}

func (m *mockWorkloadMft) ApplyEnv(envName string) (manifest.WorkloadManifest, error) {
	return m, nil
}

func (m *mockWorkloadMft) Validate() error {
	return nil
}
