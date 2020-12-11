// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/exec/mocks"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPluginCommand_StartSession(t *testing.T) {
	mockSession := &ecs.Session{
		SessionId:  aws.String("mockSessionID"),
		StreamUrl:  aws.String("mockStreamURL"),
		TokenValue: aws.String("mockTokenValue"),
	}
	var mockRunner *mocks.Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		inSession   *ecs.Session
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"return error if fail to start session": {
			inSession: mockSession,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{`{"SessionId":"mockSessionID","StreamUrl":"mockStreamURL","TokenValue":"mockTokenValue"}`, "us-west-2", "StartSession"},
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockError)
			},
			wantedError: fmt.Errorf("start session: some error"),
		},
		"success with no update and no install": {
			inSession: mockSession,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run(ssmPluginBinaryName, []string{`{"SessionId":"mockSessionID","StreamUrl":"mockStreamURL","TokenValue":"mockTokenValue"}`, "us-west-2", "StartSession"},
					gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPluginCommand{
				runner: mockRunner,
				sess: &session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				},
			}
			err := s.StartSession(tc.inSession)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSSMPluginCommand_ValidateBinary(t *testing.T) {
	const (
		mockLatestVersion  = "1.2.30.0"
		mockCurrentVersion = "1.2.7.0"
	)
	var mockRunner *mocks.Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		inLatestVersion  string
		inCurrentVersion string
		setupMocks       func(controller *gomock.Controller)
		wantedError      error
	}{
		"return error if fail to get the latest version": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, gomock.Any()).
					Return(mockError)
			},
			wantedError: fmt.Errorf("get ssm plugin latest version: some error"),
		},
		"return error if fail to get the current version": {
			inLatestVersion: mockLatestVersion,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
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
				mockRunner = mocks.NewMockrunner(controller)
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
				mockRunner = mocks.NewMockrunner(controller)
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
				mockRunner = mocks.NewMockrunner(controller)
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

func TestSSMPluginCommand_InstallLatestBinary(t *testing.T) {
	var mockDir string
	var mockRunner *mocks.Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"return error if fail to unzip binary": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("unzip", []string{"-o", filepath.Join(mockDir, "sessionmanager-bundle.zip"),
					"-d", mockDir}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to install binary": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
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
				mockRunner = mocks.NewMockrunner(controller)
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
