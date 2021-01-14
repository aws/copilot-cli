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

func TestDeleteTaskOpts_Validate(t *testing.T) {

	testCases := map[string]struct {
		inAppName        string
		inEnvName        string
		inName           string
		inDefaultCluster bool
		setupMocks       func(m *mocks.Mockstore)

		want error
	}{
		"with only app flag": {
			inAppName: "phonetool",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			want: nil,
		},
		"with no flags": {
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with app/env flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			want: nil,
		},
		"with default cluster flag set": {
			inDefaultCluster: true,
			inName:           "oneoff",
			setupMocks:       func(m *mocks.Mockstore) {},
			want:             nil,
		},
		"with default cluster and env flag": {
			inDefaultCluster: true,
			inEnvName:        "test",
			inAppName:        "phonetool",
			setupMocks:       func(m *mocks.Mockstore) {},
			want:             errors.New("cannot specify both `--app` and `--default`"),
		},
		"with error getting app": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},
			want: errors.New("get application: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)

			tc.setupMocks(mockstore)

			opts := deleteTaskOpts{
				deleteTaskVars: deleteTaskVars{
					skipConfirmation: false,
					app:              tc.inAppName,
					env:              tc.inEnvName,
					name:             tc.inName,
					defaultCluster:   tc.inDefaultCluster,
				},
				store: mockstore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.want != nil {
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

}
