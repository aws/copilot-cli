// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type secretInitMocks struct {
	mockFS    afero.Fs
	mockStore *mocks.Mockstore
}

func TestSecretInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inApp           string
		inName          string
		inValues        map[string]string
		inOverwrite     bool
		inInputFilePath string
		inResourceTags  map[string]string

		setupMocks func(m secretInitMocks)

		wantedError error
	}{
		"valid with input file": {
			inInputFilePath: "./deep/secrets.yml",
			inOverwrite:     true,
			inResourceTags: map[string]string{
				"hide": "yes",
			},
			setupMocks: func(m secretInitMocks) {
				m.mockFS.MkdirAll("deep", 0755)
				afero.WriteFile(m.mockFS, "deep/secrets.yml", []byte("FROM nginx"), 0644)
			},
		},
		"valid with name and value": {
			inName: "where_is_the_dragon",
			inValues: map[string]string{
				"good_village": "on_top_of_the_mountain",
				"bad_village":  "by_the_volcano",
			},
			inApp:       "dragon_slaying",
			inOverwrite: true,
			inResourceTags: map[string]string{
				"hide": "yes",
			},
			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_slaying").Return(&config.Application{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "good_village").Return(&config.Environment{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "bad_village").Return(&config.Environment{}, nil)
			},
		},
		"error getting app": {
			inApp: "dragon_befriending",
			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_befriending").Return(&config.Application{}, errors.New("some error"))
			},
			wantedError: errors.New("get application dragon_befriending: some error"),
		},
		"error getting env from the app": {
			inName: "where_is_the_dragon",
			inValues: map[string]string{
				"good_village":    "on_top_of_the_mountain",
				"bad_village":     "by_the_volcano",
				"neutral_village": "there_is_no_such_village",
			},
			inApp: "dragon_slaying",
			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_slaying").Return(&config.Application{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "good_village").Return(&config.Environment{}, nil).MinTimes(0).MaxTimes(1)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "bad_village").Return(&config.Environment{}, nil).MinTimes(0).MaxTimes(1)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "neutral_village").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get environment neutral_village in application dragon_slaying: some error"),
		},
		"invalid input file name": {
			inInputFilePath: "weird/path/to/secrets",
			setupMocks:      func(m secretInitMocks) {},
			wantedError:     errors.New("open weird/path/to/secrets: file does not exist"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			opts := secretInitOpts{
				secretInitVars: secretInitVars{
					appName:       tc.inApp,
					name:          tc.inName,
					values:        tc.inValues,
					inputFilePath: tc.inInputFilePath,
					overwrite:     tc.inOverwrite,
					resourceTags:  tc.inResourceTags,
				},
				fs:    &afero.Afero{Fs: afero.NewMemMapFs()},
				store: mockStore,
			}

			m := secretInitMocks{
				mockFS:    opts.fs,
				mockStore: mockStore,
			}
			tc.setupMocks(m)

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
