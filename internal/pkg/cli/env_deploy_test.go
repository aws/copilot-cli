// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
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

type deployEnvExecuteMocks struct {
	ws           *mocks.MockwsEnvironmentReader
	deployer     *mocks.MockenvDeployer
	identity     *mocks.MockidentityService
	interpolator *mocks.Mockinterpolator
}

func TestDeployEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		unmarshalManifest func(in []byte) (*manifest.Environment, error)
		setUpMocks        func(m *deployEnvExecuteMocks)
		wantedErr         error
	}{
		"fail to read manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("read manifest for environment mockEnv: some error"),
		},
		"fail to interpolate manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("", errors.New("some error"))
			},
			wantedErr: errors.New("interpolate environment variables for mockEnv manifest: some error"),
		},
		"fail to unmarshal manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("mock interpolated manifest", nil)
			},
			unmarshalManifest: func(_ []byte) (*manifest.Environment, error) {
				return nil, errors.New("some error")
			},
			wantedErr: errors.New("unmarshal environment manifest for mockEnv: some error"),
		},
		"fail to get caller identity": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("mock interpolated manifest", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))
			},
			wantedErr: errors.New("get identity: some error"),
		},
		"fail to upload manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("mock interpolated manifest", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().UploadArtifacts().Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("upload artifacts for environment mockEnv: some error"),
		},
		"fail to deploy the environment": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("mock interpolated manifest", nil)
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
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("mock manifest"), nil)
				m.interpolator.EXPECT().Interpolate("mock manifest").Return("mock interpolated manifest", nil)
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
			}
			tc.setUpMocks(m)
			opts := deployEnvOpts{
				deployEnvVars: deployEnvVars{
					name: "mockEnv",
				},
				ws:           m.ws,
				deployer:     m.deployer,
				identity:     m.identity,
				interpolator: m.interpolator,
				targetEnv: &config.Environment{
					Name: "mockEnv",
				},
				unmarshalManifest: func() func(in []byte) (*manifest.Environment, error) {
					if tc.unmarshalManifest != nil {
						return tc.unmarshalManifest
					}
					return func(_ []byte) (*manifest.Environment, error) {
						return &manifest.Environment{
							Workload: manifest.Workload{
								Name: aws.String("mockEnv"),
							},
						}, nil
					}
				}(),
			}
			err := opts.Execute()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
