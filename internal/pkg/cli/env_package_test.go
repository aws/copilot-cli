// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"io"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"

	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
)

func TestPackageEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		in        packageEnvVars
		mockedCmd func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts

		wanted error
	}{
		"should return errNoAppInWorkspace if app name is empty": {
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				return &packageEnvOpts{
					packageEnvVars: vars,
				}
			},
			wanted: errNoAppInWorkspace,
		},
		"should return a wrapped error if application name cannot be retrieved": {
			in: packageEnvVars{
				appName: "phonetool",
			},
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				cfgStore := mocks.NewMockstore(ctrl)
				cfgStore.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
				return &packageEnvOpts{
					packageEnvVars: vars,
					cfgStore:       cfgStore,
				}
			},

			wanted: errors.New(`get application "phonetool" configuration: some error`),
		},
		"should return a wrapped error if environment name doesn't exist in SSM": {
			in: packageEnvVars{
				appName: "phonetool",
				envName: "test",
			},
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				cfgStore := mocks.NewMockstore(ctrl)
				cfgStore.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
				cfgStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				return &packageEnvOpts{
					packageEnvVars: vars,
					cfgStore:       cfgStore,
				}
			},

			wanted: errors.New(`get environment "test" in application "phonetool": some error`),
		},
		"should return a wrapped error if environment cannot be selected from workspace": {
			in: packageEnvVars{
				appName: "phonetool",
			},
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				cfgStore := mocks.NewMockstore(ctrl)
				cfgStore.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
				sel := mocks.NewMockwsEnvironmentSelector(ctrl)
				sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Return("", errors.New("no environments found"))
				return &packageEnvOpts{
					packageEnvVars: vars,
					cfgStore:       cfgStore,
					sel:            sel,
				}
			},

			wanted: errors.New(`select environment: no environments found`),
		},
		"should return nil if environment name was asked successfully": {
			in: packageEnvVars{
				appName: "phonetool",
			},
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				cfgStore := mocks.NewMockstore(ctrl)
				cfgStore.EXPECT().GetApplication(vars.appName).Return(&config.Application{}, nil)
				sel := mocks.NewMockwsEnvironmentSelector(ctrl)
				sel.EXPECT().LocalEnvironment("Select an environment manifest from your workspace", "").Return("test", nil)
				return &packageEnvOpts{
					packageEnvVars: vars,
					cfgStore:       cfgStore,
					sel:            sel,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cmd := tc.mockedCmd(ctrl, tc.in)

			// WHEN
			actual := cmd.Ask()

			// THEN
			if tc.wanted == nil {
				require.NoError(t, actual)
			} else {
				require.EqualError(t, actual, tc.wanted.Error())
			}
		})
	}
}

func TestPackageEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		mockedCmd func(controller *gomock.Controller) *packageEnvOpts

		wantedFS  func(t *testing.T, fs afero.Fs)
		wantedErr error
	}{
		"should return a wrapped error when reading env manifest fails": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return(nil, errors.New("some error"))
				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName: "test",
					},
					ws: ws,
					newInterpolator: func(_, _ string) interpolator {
						return nil
					},
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedErr: errors.New(`read manifest for environment "test": some error`),
		},
		"should return a wrapped error when manifest interpolation fails": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("hi"), nil)
				interop := mocks.NewMockinterpolator(ctrl)
				interop.EXPECT().Interpolate(gomock.Any()).Return("", errors.New("some error"))

				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName: "test",
					},
					ws: ws,
					newInterpolator: func(_, _ string) interpolator {
						return interop
					},
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedErr: errors.New(`interpolate environment variables for "test" manifest: some error`),
		},
		"should return a wrapped error when STS call fails": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: test\ntype: Environment\n"), nil)
				interop := mocks.NewMockinterpolator(ctrl)
				interop.EXPECT().Interpolate(gomock.Any()).Return("name: test\ntype: Environment\n", nil)
				caller := mocks.NewMockidentityService(ctrl)
				caller.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))

				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName: "test",
					},
					ws:     ws,
					caller: caller,
					newInterpolator: func(_, _ string) interpolator {
						return interop
					},
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedErr: errors.New(`get caller principal identity: some error`),
		},
		"should return a wrapped error when uploading assets fails": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: test\ntype: Environment\n"), nil)
				interop := mocks.NewMockinterpolator(ctrl)
				interop.EXPECT().Interpolate(gomock.Any()).Return("name: test\ntype: Environment\n", nil)
				caller := mocks.NewMockidentityService(ctrl)
				caller.EXPECT().Get().Return(identity.Caller{}, nil)
				deployer := mocks.NewMockenvPackager(ctrl)
				deployer.EXPECT().UploadArtifacts().Return(nil, errors.New("some error"))

				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName:      "test",
						uploadAssets: true,
					},
					ws:     ws,
					caller: caller,
					newInterpolator: func(_, _ string) interpolator {
						return interop
					},
					newEnvDeployer: func() (envPackager, error) {
						return deployer, nil
					},
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedErr: errors.New(`upload assets for environment "test": some error`),
		},
		"should return a wrapped error when generating CloudFormation templates fails": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: test\ntype: Environment\n"), nil)
				interop := mocks.NewMockinterpolator(ctrl)
				interop.EXPECT().Interpolate(gomock.Any()).Return("name: test\ntype: Environment\n", nil)
				caller := mocks.NewMockidentityService(ctrl)
				caller.EXPECT().Get().Return(identity.Caller{}, nil)
				deployer := mocks.NewMockenvPackager(ctrl)
				deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(nil, errors.New("some error"))

				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName: "test",
					},
					ws:     ws,
					caller: caller,
					newInterpolator: func(_, _ string) interpolator {
						return interop
					},
					newEnvDeployer: func() (envPackager, error) {
						return deployer, nil
					},
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedErr: errors.New(`generate CloudFormation template from environment "test" manifest: some error`),
		},
		"should write files to output directories": {
			mockedCmd: func(ctrl *gomock.Controller) *packageEnvOpts {
				ws := mocks.NewMockwsEnvironmentReader(ctrl)
				ws.EXPECT().ReadEnvironmentManifest("test").Return([]byte("name: test\ntype: Environment\n"), nil)
				interop := mocks.NewMockinterpolator(ctrl)
				interop.EXPECT().Interpolate("name: test\ntype: Environment\n").Return("name: test\ntype: Environment\n", nil)
				caller := mocks.NewMockidentityService(ctrl)
				caller.EXPECT().Get().Return(identity.Caller{}, nil)
				deployer := mocks.NewMockenvPackager(ctrl)
				deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{
					Template:   "template",
					Parameters: "parameters",
				}, nil)

				fs := afero.NewMemMapFs()

				return &packageEnvOpts{
					packageEnvVars: packageEnvVars{
						envName:   "test",
						outputDir: "infrastructure",
					},
					ws:     ws,
					caller: caller,
					newInterpolator: func(_, _ string) interpolator {
						return interop
					},
					newEnvDeployer: func() (envPackager, error) {
						return deployer, nil
					},
					fs:     fs,
					envCfg: &config.Environment{Name: "test"},
				}
			},
			wantedFS: func(t *testing.T, fs afero.Fs) {
				f, err := fs.Open("infrastructure/test.env.yml")
				require.NoError(t, err)
				actual, err := io.ReadAll(f)
				require.NoError(t, err)
				require.Equal(t, []byte("template"), actual)

				f, err = fs.Open("infrastructure/test.env.params.json")
				require.NoError(t, err)
				actual, err = io.ReadAll(f)
				require.NoError(t, err)
				require.Equal(t, []byte("parameters"), actual)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cmd := tc.mockedCmd(ctrl)

			// WHEN
			actual := cmd.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, actual)
				tc.wantedFS(t, cmd.fs)
			} else {
				require.EqualError(t, actual, tc.wantedErr.Error())
			}
		})
	}
}
