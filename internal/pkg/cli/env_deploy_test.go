// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deployEnvAskMocks struct {
	ws    *mocks.MockwsEnvironmentReader
	sel   *mocks.MockwsEnvironmentSelector
	store *mocks.Mockstore
}

func TestDeployEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inName     string
		setUpMocks func(m *deployEnvAskMocks)

		wantedEnvName string
		wantedError   error
	}{
		"fail to retrieve app from store when validating app": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get application mockApp: some error"),
		},
		"error if no app in workspace": {
			inName: "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Times(0)
			},
			wantedError: errNoAppInWorkspace,
		},
		"fail to list environments in local workspace": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("list environments in workspace: some error"),
		},
		"fail to find local environment manifest workspace": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"otherEnv"}, nil)
			},
			wantedError: errors.New(`environment manifest for "mockEnv" is not found`),
		},
		"fail to retrieve env from store when validating env": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get environment mockEnv in application mockApp: some error"),
		},
		"fail to ask for an env from workspace": {
			inAppName: "mockApp",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").AnyTimes()
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("select environment: some error"),
		},
		"validate env instead of asking if it is passed in as a flag": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{}, nil)
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedEnvName: "mockEnv",
		},
		"ask for env": {
			inAppName: "mockApp",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.ws.EXPECT().ListEnvironments().Times(0)
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Return("mockEnv", nil)
			},
			wantedEnvName: "mockEnv",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &deployEnvAskMocks{
				ws:    mocks.NewMockwsEnvironmentReader(ctrl),
				sel:   mocks.NewMockwsEnvironmentSelector(ctrl),
				store: mocks.NewMockstore(ctrl),
			}
			tc.setUpMocks(m)
			opts := deployEnvOpts{
				deployEnvVars: deployEnvVars{
					appName: tc.inAppName,
					name:    tc.inName,
				},
				ws:    m.ws,
				sel:   m.sel,
				store: m.store,
			}
			gotErr := opts.Ask()
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, opts.name, tc.wantedEnvName)
			}
		})
	}
}

type deployEnvExecuteMocks struct {
	ws           *mocks.MockwsEnvironmentReader
	deployer     *mocks.MockenvDeployer
	identity     *mocks.MockidentityService
	interpolator *mocks.Mockinterpolator
	describer    *mocks.MockenvDescriber
}

func TestDeployEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		unmarshalManifest func(in []byte) (*manifest.Environment, error)
		setUpMocks        func(m *deployEnvExecuteMocks)
		wantedErr         error
	}{
		"fail to read manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New(`read manifest for environment "mockEnv": some error`),
		},
		"fail to interpolate manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New(`interpolate environment variables for "mockEnv" manifest: some error`),
		},
		"fail to unmarshal manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("failing manifest format", nil)
			},
			wantedErr: errors.New(`unmarshal environment manifest for "mockEnv"`),
		},
		"fail to validate cdn": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\ncdn: true\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\ncdn: true\n", nil)
				m.describer.EXPECT().ValidateCFServiceDomainAliases().Return(fmt.Errorf("mock error"))
			},
			wantedErr: errors.New("mock error"),
		},
		"fail to get caller identity": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))
			},
			wantedErr: errors.New("get identity: some error"),
		},
		"fail to upload manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().UploadArtifacts().Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("upload artifacts for environment mockEnv: some error"),
		},
		"fail to deploy the environment": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().UploadArtifacts().Return(map[string]string{
					"mockResource": "mockURL",
				}, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).DoAndReturn(func(_ *deploy.DeployEnvironmentInput) error {
					return errors.New("some error")
				})
			},
			wantedErr: errors.New("deploy environment mockEnv: some error"),
		},
		"success": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate("name: mockEnv\ntype: Environment\n").Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().UploadArtifacts().Return(map[string]string{
					"mockResource": "mockURL",
				}, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).DoAndReturn(func(in *deploy.DeployEnvironmentInput) error {
					require.Equal(t, in.RootUserARN, "mockRootUserARN")
					require.Equal(t, in.CustomResourcesURLs, map[string]string{
						"mockResource": "mockURL",
					})
					require.Equal(t, in.Manifest, &manifest.Environment{
						Workload: manifest.Workload{
							Name: aws.String("mockEnv"),
							Type: aws.String("Environment"),
						},
					})
					return nil
				})
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &deployEnvExecuteMocks{
				ws:           mocks.NewMockwsEnvironmentReader(ctrl),
				deployer:     mocks.NewMockenvDeployer(ctrl),
				identity:     mocks.NewMockidentityService(ctrl),
				interpolator: mocks.NewMockinterpolator(ctrl),
				describer:    mocks.NewMockenvDescriber(ctrl),
			}
			tc.setUpMocks(m)
			opts := deployEnvOpts{
				deployEnvVars: deployEnvVars{
					name: "mockEnv",
				},
				ws:       m.ws,
				identity: m.identity,
				newEnvDeployer: func() (envDeployer, error) {
					return m.deployer, nil
				},
				newInterpolator: func(s string, s2 string) interpolator {
					return m.interpolator
				},
				newEnvDescriber: func() (envDescriber, error) {
					return m.describer, nil
				},
				targetApp: &config.Application{
					Name:   "mockApp",
					Domain: "mockDomain",
				},
				targetEnv: &config.Environment{
					Name: "mockEnv",
				},
			}
			err := opts.Execute()
			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
