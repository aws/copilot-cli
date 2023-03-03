// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestOverrideSvcOpts_Validate(t *testing.T) {
	t.Run("validate application", func(t *testing.T) {
		testCases := map[string]struct {
			appName   string
			initMocks func(ctrl *gomock.Controller, cmd *overrideSvcOpts)

			wanted error
		}{
			"return an error if the workspace has no application associated with": {
				wanted: errNoAppInWorkspace,
			},
			"return a wrapped error if the workspace's application cannot be fetched from the Config Store": {
				appName: "demo",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
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

				in := overrideVars{appName: tc.appName}
				cmd := &overrideSvcOpts{overrideVars: in}
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

				in := overrideVars{cdkLang: tc.lang, appName: "demo"}
				cmd := &overrideSvcOpts{
					overrideVars: in,
					cfgStore:     mockSSM,
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

func TestOverrideSvcOpts_Ask(t *testing.T) {
	t.Run("ask or validate service name", func(t *testing.T) {
		testCases := map[string]struct {
			name      string
			initMocks func(ctrl *gomock.Controller, cmd *overrideSvcOpts)

			wanted error
		}{
			"validation passes if service exists in local workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return([]string{"backend", "frontend", "worker"}, nil)
					cmd.ws = mockWS
				},
			},
			"return a wrapped error if local services cannot be retrieved from workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return(nil, errors.New("some error"))
					cmd.ws = mockWS
				},
				wanted: errors.New("list services in the workspace: some error"),
			},
			"return an error if service does not exist in the workspace": {
				name: "frontend",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListServices().Return([]string{"backend"}, nil)
					cmd.ws = mockWS
				},
				wanted: errors.New(`service "frontend" does not exist in the workspace`),
			},
			"should ask for the local service name if flag is not provided": {
				name: "",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
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

				in := overrideVars{name: tc.name, appName: "demo", iacTool: "cdk"}
				cmd := &overrideSvcOpts{
					overrideVars: in,
					cfgStore:     mocks.NewMockstore(ctrl),
					cfnPrompt:    mockCfnPrompt,
					packageCmd: func(_ stringWriteCloser) (executor, error) {
						mockCmd := mocks.NewMockexecutor(ctrl)
						mockCmd.EXPECT().Execute().AnyTimes()
						return mockCmd, nil
					},
				}
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
			initMocks func(ctrl *gomock.Controller, cmd *overrideSvcOpts)

			wanted error
		}{
			"validation passes if IaC tool is a valid option": {
				iacTool: "cdk",
				wanted:  nil,
			},
			"return an error if IaC tool flag value is invalid": {
				iacTool: "terraform",
				wanted:  errors.New(`"terraform" is not a valid IaC tool: must be one of: "cdk"`),
			},
			"should ask for IaC tool name if flag is not provided": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockPrompt := mocks.NewMockprompter(ctrl)
					mockPrompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), []string{"cdk"}, gomock.Any())
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

				in := overrideVars{iacTool: tc.iacTool, name: "frontend", appName: "demo"}
				cmd := &overrideSvcOpts{
					overrideVars: in,
					cfgStore:     mockSSM,
					ws:           mockWS,
					cfnPrompt:    mockCfnPrompt,
					packageCmd: func(_ stringWriteCloser) (executor, error) {
						mockCmd := mocks.NewMockexecutor(ctrl)
						mockCmd.EXPECT().Execute().AnyTimes()
						return mockCmd, nil
					},
				}
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
			initMocks func(ctrl *gomock.Controller, cmd *overrideSvcOpts)
			wanted    error
		}{
			"should return an error if package command cannot be initialized": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					cmd.packageCmd = func(_ stringWriteCloser) (executor, error) {
						return nil, errors.New("init fail")
					}
				},
				wanted: errors.New("init fail"),
			},
			"should return a wrapped error if package command fails to execute": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockPkgCmd := mocks.NewMockexecutor(ctrl)
					mockPkgCmd.EXPECT().Execute().Return(errors.New("some error"))
					cmd.packageCmd = func(_ stringWriteCloser) (executor, error) {
						return mockPkgCmd, nil
					}
				},
				wanted: errors.New(`generate CloudFormation template for service "frontend": some error`),
			},
			"should prompt for CloudFormation resources in a template": {
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
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

				in := overrideVars{iacTool: "cdk", name: "frontend", appName: "demo"}
				cmd := &overrideSvcOpts{
					overrideVars: in,
					cfgStore:     mockSSM,
					ws:           mockWS,
				}
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
