// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testError = errors.New("some error")

type localRunAskMocks struct {
	store *mocks.Mockstore
}

func TestLocalRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputAppName  string
		setupMocks    func(m *localRunAskMocks)
		wantedAppName string
		wantedError   error
	}{
		"no app in workspace": {
			wantedError: errNoAppInWorkspace,
		},
		"fail to read the application from SSM store": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(nil, testError)
			},
			wantedError: fmt.Errorf("get application testApp: %w", testError),
		},
		"successful validation": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(&config.Application{Name: "testApp"}, nil)
			},
			wantedAppName: "testApp",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName: tc.inputAppName,
				},
				store: m.store,
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
