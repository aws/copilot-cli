// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
)

func TestPackageEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		in        packageEnvVars
		mockedCmd func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts

		wantedErr error
	}{
		"should return errNoAppInWorkspace if app name is empty": {
			mockedCmd: func(ctrl *gomock.Controller, vars packageEnvVars) *packageEnvOpts {
				return &packageEnvOpts{
					packageEnvVars: vars,
				}
			},
			wantedErr: errNoAppInWorkspace,
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

			wantedErr: errors.New(`get application "phonetool" configuration: some error`),
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

			wantedErr: errors.New(`get environment "test" in application "phonetool": some error`),
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

			wantedErr: errors.New(`select environment: no environments found`),
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
			if tc.wantedErr == nil {
				require.NoError(t, actual)
			} else {
				require.EqualError(t, actual, tc.wantedErr.Error())
			}
		})
	}
}
