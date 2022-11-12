// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPluginCommand_InstallLatestBinary_darwin(t *testing.T) {
	var mockDir string
	var mockRunner *Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"return error if fail to unzip binary": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("unzip", []string{"-o", filepath.Join(mockDir, "sessionmanager-bundle.zip"),
					"-d", mockDir}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to install binary": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("unzip", []string{"-o", filepath.Join(mockDir, "sessionmanager-bundle.zip"),
					"-d", mockDir}).
					Return(nil)
				mockRunner.EXPECT().Run("sudo", []string{filepath.Join(mockDir, "sessionmanager-bundle", "install"), "-i",
					"/usr/local/sessionmanagerplugin", "-b",
					"/usr/local/bin/session-manager-plugin"}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("unzip", []string{"-o", filepath.Join(mockDir, "sessionmanager-bundle.zip"),
					"-d", mockDir}).
					Return(nil)
				mockRunner.EXPECT().Run("sudo", []string{filepath.Join(mockDir, "sessionmanager-bundle", "install"), "-i",
					"/usr/local/sessionmanagerplugin", "-b",
					"/usr/local/bin/session-manager-plugin"}).
					Return(nil)
			},
		},
	}
	for name, tc := range tests {
		mockDir, _ = os.MkdirTemp("", "temp")
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPluginCommand{
				runner:  mockRunner,
				tempDir: mockDir,
				http: &fakeHTTPClient{
					content: []byte("hello"),
				},
			}
			err := s.InstallLatestBinary()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
		os.RemoveAll(mockDir)
	}
}
