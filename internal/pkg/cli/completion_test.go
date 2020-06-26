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

func TestCompletionOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputShell  string
		wantedError error
	}{
		"zsh": {
			inputShell:  "zsh",
			wantedError: nil,
		},
		"bash": {
			inputShell:  "bash",
			wantedError: nil,
		},
		"invalid shell": {
			inputShell:  "chicken",
			wantedError: errors.New("shell must be bash or zsh"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := completionOpts{Shell: tc.inputShell}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestCompletionOpts_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputShell  string
		mocking     func(mock *mocks.MockshellCompleter)
		wantedError error
	}{
		"bash": {
			inputShell: "bash",
			mocking: func(mock *mocks.MockshellCompleter) {
				mock.EXPECT().GenBashCompletion(gomock.Any()).Times(1)
				mock.EXPECT().GenZshCompletion(gomock.Any()).Times(0)
			},
		},
		"zsh": {
			inputShell: "zsh",
			mocking: func(mock *mocks.MockshellCompleter) {
				mock.EXPECT().GenBashCompletion(gomock.Any()).Times(0)
				mock.EXPECT().GenZshCompletion(gomock.Any()).Times(1)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mock := mocks.NewMockshellCompleter(ctrl)
			tc.mocking(mock)
			opts := completionOpts{Shell: tc.inputShell, completer: mock}

			// WHEN
			opts.Execute()
		})
	}
}
