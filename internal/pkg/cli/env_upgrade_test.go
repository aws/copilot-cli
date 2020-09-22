// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
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
				m := mocks.NewMockenvironmentStore(ctrl)
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
				m := mocks.NewMockenvironmentStore(ctrl)
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
				m := mocks.NewMockenvironmentStore(ctrl)
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
to support latest Copilot features.`,
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
