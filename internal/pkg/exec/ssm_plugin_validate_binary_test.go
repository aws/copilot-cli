//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPluginCommand_ValidateBinary(t *testing.T) {
	const (
		mockLatestVersion  = "1.2.30.0"
		mockCurrentVersion = "1.2.7.0"
	)
	var mockRunner *Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		inLatestVersion  string
		inCurrentVersion string
		setupMocks       func(controller *gomock.Controller)
		wantedError      error
	}{
		"return error if fail to get the latest version": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(mockError)
			},
			wantedError: fmt.Errorf("get ssm plugin latest version: some error"),
		},
		"return error if fail to get the current version": {
			inLatestVersion: mockLatestVersion,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(nil)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{"--version"}, gomock.Any()).
					Return(mockError)
			},
			wantedError: fmt.Errorf("get local ssm plugin version: some error"),
		},
		"return ErrSSMPluginNotExist if plugin doesn't exist": {
			inLatestVersion: mockLatestVersion,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(nil)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{"--version"}, gomock.Any()).
					Return(errors.New("executable file not found in $PATH"))
			},
			wantedError: fmt.Errorf("Session Manager plugin does not exist"),
		},
		"return ErrOutdatedSSMPlugin if plugin needs update": {
			inLatestVersion:  mockLatestVersion,
			inCurrentVersion: mockCurrentVersion,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(nil)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{"--version"}, gomock.Any()).
					Return(nil)
			},
			wantedError: fmt.Errorf("Session Manager plugin is not up-to-date"),
		},
		"return nil if no update needed": {
			inLatestVersion:  mockLatestVersion,
			inCurrentVersion: mockLatestVersion,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(nil)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{"--version"}, gomock.Any()).
					Return(nil)
			},
			wantedError: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPluginCommand{
				runner:               mockRunner,
				currentVersionBuffer: *bytes.NewBufferString(tc.inCurrentVersion),
				latestVersionBuffer:  *bytes.NewBufferString(tc.inLatestVersion),
			}
			err := s.ValidateBinary()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
