package cli

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLocalRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		wantedErr error
	}{
		"no app in workspace": {
			wantedErr: errNoAppInWorkspace,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := localRunOpts{}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
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

		setupMocks     func(m *mocks.MockwsSelector)
		wantedWkldName string
		wantedEnvName  string
		wantedError    error
	}{
		"prompts for environment name and workload names": {
			inputAppName: testAppName,
			setupMocks: func(m *mocks.MockwsSelector) {
				m.EXPECT().Environment("Select an environment", "", "testApp").Return("testEnv", nil)
				m.EXPECT().Workload("Select a workload from your workspace", "").Return("testWkld", nil)
			},

			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"don't call selector if flags are provided": {
			inputAppName:  testAppName,
			inputEnvName:  testEnvName,
			inputWkldName: testWkldName,
			setupMocks: func(m *mocks.MockwsSelector) {
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Times(0)
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
			mockSel := mocks.NewMockwsSelector(ctrl)

			tc.setupMocks(mockSel)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName: tc.inputAppName,
					name:    tc.inputWkldName,
					envName: tc.inputEnvName,
				},
				sel: mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWkldName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}
