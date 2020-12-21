// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/exec/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPluginCommand_InstallLatestBinary_linux(t *testing.T) {
	var mockDir string
	var mockRunner *mocks.Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"return error if fail to install binary": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("sudo", []string{"yum", "install", "-y",
					filepath.Join(mockDir, "session-manager-plugin.rpm")}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("sudo", []string{"yum", "install", "-y",
					filepath.Join(mockDir, "session-manager-plugin.rpm")}).
					Return(nil)
			},
		},
	}
	for name, tc := range tests {
		mockDir, _ = ioutil.TempDir("", "temp")
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPluginCommand{
				runner:  mockRunner,
				tempDir: mockDir,
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
