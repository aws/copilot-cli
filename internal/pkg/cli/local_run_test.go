// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type localRunAskMocks struct {
	store *mocks.Mockstore
	sel   *mocks.MockwsSelector
}

func TestLocalRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		wantedError error
	}{
		"no app in workspace": {
			wantedError: errNoAppInWorkspace,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := localRunOpts{}

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

func TestLocalRunOpts_Ask(t *testing.T) {
	const (
		testAppName  = "testApp"
		testEnvName  = "testEnv"
		testWkldName = "testWkld"
	)
	testCases := map[string]struct {
		inputAppName  string
		inputEnvName  string
		inputWkldName string

		setupMocks     func(m *localRunAskMocks)
		wantedWkldName string
		wantedEnvName  string
		wantedError    error
	}{
		"validate if flags are provided": {
			inputAppName:  testAppName,
			inputEnvName:  testEnvName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetWorkload("testApp", "testWkld").Return(&config.Workload{}, nil)
				m.sel.EXPECT().Workload(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(&config.Environment{Name: "testEnv"}, nil)
				m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"prompts for environment name and workload names": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.sel.EXPECT().Workload("Select a workload from your workspace that you want to run locally", "").Return("testWkld", nil)
				m.sel.EXPECT().Environment("Select an environment", "", "testApp").Return("testEnv", nil)
			},

			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"prompt for workload name": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.sel.EXPECT().Workload("Select a workload from your workspace that you want to run locally", "").Return("testWkld", nil)
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(&config.Environment{Name: "testEnv"}, nil)
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"prompt for environment name": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetWorkload("testApp", "testWkld").Return(&config.Workload{}, nil)
				m.sel.EXPECT().Environment("Select an environment", "", "testApp").Return("testEnv", nil)
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockwsSelector(ctrl),
			}
			tc.setupMocks(m)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName:  tc.inputAppName,
					wkldName: tc.inputWkldName,
					envName:  tc.inputEnvName,
				},
				sel:   m.sel,
				store: m.store,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWkldName, opts.wkldName)
				require.Equal(t, tc.wantedEnvName, opts.envName)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}
