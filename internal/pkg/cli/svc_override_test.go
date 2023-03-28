// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestOverrideSvc_Validate(t *testing.T) {
	t.Run("validate application", func(t *testing.T) {
		testCases := map[string]struct {
			appName   string
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)

			wanted error
		}{
			"return an error if the workspace has no application associated with": {
				wanted: errNoAppInWorkspace,
			},
			"return a wrapped error if the workspace's application cannot be fetched from the Config Store": {
				appName: "demo",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockSSM := mocks.NewMockstore(ctrl)
					mockSSM.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
					cmd.cfgStore = mockSSM
				},
				wanted: errors.New(`get application "demo" configuration: some error`),
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				vars := overrideVars{appName: tc.appName}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
					},
				}
				if tc.initMocks != nil {
					tc.initMocks(ctrl, cmd)
				}

				// WHEN
				err := cmd.Validate()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
	t.Run("validate environment name", func(t *testing.T) {
		testCases := map[string]struct {
			envName   string
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)

			wanted error
		}{
			"skip validating if environment name is empty": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockSSM := mocks.NewMockstore(ctrl)
					mockSSM.EXPECT().GetApplication(gomock.Any()).AnyTimes()
					cmd.cfgStore = mockSSM
				},
			},
			"return a wrapped error if the environment flag value does not exist": {
				envName: "test",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockSSM := mocks.NewMockstore(ctrl)
					mockSSM.EXPECT().GetApplication(gomock.Any()).AnyTimes()
					mockSSM.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, &config.ErrNoSuchEnvironment{})
					cmd.cfgStore = mockSSM
				},
				wanted: errors.New(`get environment "test" configuration`),
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				vars := overrideVars{appName: "demo", cdkLang: "typescript"}
				cmd := &overrideWorkloadOpts{
					envName: tc.envName,
					overrideOpts: &overrideOpts{
						overrideVars: vars,
					},
				}
				if tc.initMocks != nil {
					tc.initMocks(ctrl, cmd)
				}

				// WHEN
				err := cmd.Validate()

				// THEN
				if tc.wanted != nil {
					require.ErrorContains(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
	t.Run("validate CDK language", func(t *testing.T) {
		testCases := map[string]struct {
			lang   string
			wanted error
		}{
			"return an error when an unknown language is selected": {
				lang:   "python",
				wanted: errors.New(`"python" is not a valid CDK language: must be one of: "typescript"`),
			},
			"typescript is a valid CDK language": {
				lang: "typescript",
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockSSM := mocks.NewMockstore(ctrl)
				mockSSM.EXPECT().GetApplication(gomock.Any()).Return(nil, nil)

				vars := overrideVars{appName: "demo", cdkLang: tc.lang}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
						cfgStore:     mockSSM,
					},
				}

				// WHEN
				err := cmd.Validate()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestOverrideSvc_Ask(t *testing.T) {
	t.Run("ask or validate service name", func(t *testing.T) {
		testCases := map[string]struct {
			name      string
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)

			wanted error
		}{
			"validation passes if service exists in local workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return([]string{"backend", "frontend", "worker"}, nil)
					cmd.ws = mockWS
				},
			},
			"return a wrapped error if local services cannot be retrieved from workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return(nil, errors.New("some error"))
					cmd.ws = mockWS
				},
				wanted: errors.New("list services in the workspace: some error"),
			},
			"return an error if service does not exist in the workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return([]string{"backend"}, nil)
					cmd.ws = mockWS
				},
				wanted: errors.New(`service "frontend" does not exist in the workspace`),
			},
			"should ask for the local service name if flag is not provided": {
				name: "",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPrompt := mocks.NewMockwsSelector(ctrl)
					mockPrompt.EXPECT().Service(gomock.Any(), gomock.Any())
					cmd.wsPrompt = mockPrompt
				},
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockCfnPrompt := mocks.NewMockcfnSelector(ctrl)
				mockCfnPrompt.EXPECT().Resources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				vars := overrideVars{name: tc.name, appName: "demo", iacTool: "cdk"}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
						cfgStore:     mocks.NewMockstore(ctrl),
						cfnPrompt:    mockCfnPrompt,
						packageCmd: func(_ stringWriteCloser) (executor, error) {
							mockCmd := mocks.NewMockexecutor(ctrl)
							mockCmd.EXPECT().Execute().AnyTimes()
							return mockCmd, nil
						},
						spinner: &spinnerTestDouble{},
					},
				}
				cmd.validateOrAskName = cmd.validateOrAskServiceName
				tc.initMocks(ctrl, cmd)

				// WHEN
				err := cmd.Ask()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
	t.Run("ask or validate IaC tool", func(t *testing.T) {
		testCases := map[string]struct {
			iacTool   string
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)

			wanted error
		}{
			"validation passes if IaC tool is a valid option": {
				iacTool: "cdk",
				wanted:  nil,
			},
			"return an error if IaC tool flag value is invalid": {
				iacTool: "terraform",
				wanted:  errors.New(`"terraform" is not a valid IaC tool: must be one of: "cdk", "yamlpatch"`),
			},
			"should ask for IaC tool name if flag is not provided": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPrompt := mocks.NewMockprompter(ctrl)
					mockPrompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), []string{"cdk", "yamlpatch"}, gomock.Any())
					cmd.prompt = mockPrompt
				},
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockSSM := mocks.NewMockstore(ctrl)
				mockWS := mocks.NewMockwsWlDirReader(ctrl)
				mockWS.EXPECT().ListServices().Return([]string{"frontend"}, nil).AnyTimes()
				mockCfnPrompt := mocks.NewMockcfnSelector(ctrl)
				mockCfnPrompt.EXPECT().Resources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				vars := overrideVars{appName: "demo", name: "frontend", iacTool: tc.iacTool}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
						cfgStore:     mockSSM,
						cfnPrompt:    mockCfnPrompt,
						packageCmd: func(_ stringWriteCloser) (executor, error) {
							mockCmd := mocks.NewMockexecutor(ctrl)
							mockCmd.EXPECT().Execute().AnyTimes()
							return mockCmd, nil
						},
						spinner: &spinnerTestDouble{},
					},
					ws: mockWS,
				}
				cmd.validateOrAskName = cmd.validateOrAskServiceName
				if tc.initMocks != nil {
					tc.initMocks(ctrl, cmd)
				}

				// WHEN
				err := cmd.Ask()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
	t.Run("ask for which template resources to override", func(t *testing.T) {
		testCases := map[string]struct {
			skip      bool
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)
			wanted    error
		}{
			"should skip prompting for resources if the user opts-in to generating empty files": {
				skip: true,
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPrompt := mocks.NewMockcfnSelector(ctrl)
					mockPrompt.EXPECT().Resources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
					cmd.cfnPrompt = mockPrompt
				},
			},
			"should return an error if package command cannot be initialized": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					cmd.packageCmd = func(_ stringWriteCloser) (executor, error) {
						return nil, errors.New("init fail")
					}
				},
				wanted: errors.New("init fail"),
			},
			"should return a wrapped error if package command fails to execute": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPkgCmd := mocks.NewMockexecutor(ctrl)
					mockPkgCmd.EXPECT().Execute().Return(errors.New("some error"))
					cmd.packageCmd = func(_ stringWriteCloser) (executor, error) {
						return mockPkgCmd, nil
					}
				},
				wanted: errors.New(`generate CloudFormation template for "frontend": some error`),
			},
			"should prompt for CloudFormation resources in a template": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPkgCmd := mocks.NewMockexecutor(ctrl)
					mockPkgCmd.EXPECT().Execute().Return(nil)
					mockPrompt := mocks.NewMockcfnSelector(ctrl)
					template := `
Resources:
  Queue:
    Type: AWS::SQS::Queue
  Service:
    Type: AWS::ECS::Service
`
					mockPrompt.EXPECT().Resources(gomock.Any(), gomock.Any(), gomock.Any(), template).Return(nil, nil)

					cmd.packageCmd = func(w stringWriteCloser) (executor, error) {
						_, _ = w.Write([]byte(template))
						return mockPkgCmd, nil
					}
					cmd.cfnPrompt = mockPrompt
				},
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockSSM := mocks.NewMockstore(ctrl)
				mockWS := mocks.NewMockwsWlDirReader(ctrl)
				mockWS.EXPECT().ListServices().Return([]string{"frontend"}, nil).AnyTimes()

				vars := overrideVars{appName: "demo", name: "frontend", iacTool: "cdk", skipResources: tc.skip}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
						cfgStore:     mockSSM,
						spinner:      &spinnerTestDouble{},
					},
					ws: mockWS,
				}
				cmd.validateOrAskName = cmd.validateOrAskServiceName
				tc.initMocks(ctrl, cmd)

				// WHEN
				err := cmd.Ask()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestOverrideSvc_Execute(t *testing.T) {
	t.Run("with the CDK", func(t *testing.T) {
		testCases := map[string]struct {
			resources []template.CFNResource
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)
			wanted    error
		}{
			"should succeed creating IaC files without any resources": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					fs := afero.NewMemMapFs()
					ws := mocks.NewMockwsWlDirReader(ctrl)
					ws.EXPECT().WorkloadOverridesPath(gomock.Any()).Return(filepath.Join("copilot", "frontend", "overrides"))
					cmd.ws = ws
					cmd.fs = fs
				},
			},
			"should succeed creating IaC files with resources": {
				resources: []template.CFNResource{
					{
						Type:      "AWS::ECS::Service",
						LogicalID: "Service",
					},
				},
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					fs := afero.NewMemMapFs()
					ws := mocks.NewMockwsWlDirReader(ctrl)
					ws.EXPECT().WorkloadOverridesPath(gomock.Any()).Return(filepath.Join("copilot", "frontend", "overrides"))
					cmd.ws = ws
					cmd.fs = fs
				},
			},
			"should return a wrapped error if override files already exists": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					dir := filepath.Join("copilot", "frontend", "overrides")
					fs := afero.NewMemMapFs()
					_ = fs.MkdirAll(dir, 0755)
					_ = afero.WriteFile(fs, filepath.Join(dir, "cdk.json"), []byte("content"), 0755)
					ws := mocks.NewMockwsWlDirReader(ctrl)
					ws.EXPECT().WorkloadOverridesPath(gomock.Any()).Return(dir)
					cmd.ws = ws
					cmd.fs = fs
				},
				wanted: fmt.Errorf("scaffold CDK application under %q", filepath.Join("copilot", "frontend", "overrides")),
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				vars := overrideVars{appName: "demo", name: "frontend", iacTool: "cdk", resources: tc.resources}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
					},
				}
				tc.initMocks(ctrl, cmd)

				// WHEN
				err := cmd.Execute()

				// THEN
				if tc.wanted != nil {
					require.ErrorContains(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
