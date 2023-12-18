// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPluginCommand_InstallLatestBinary_linux(t *testing.T) {
	var mockDir string
	var mockRunner *Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		setupMocks   func(controller *gomock.Controller)
		linuxVersion string
		wantedError  error
	}{
		"return error if fail to check linux distribution": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("cat", []string{"/etc/os-release"}, gomock.Any()).
					Return(mockError)
			},
			wantedError: fmt.Errorf("run cat /etc/os-release: some error"),
		},
		"return error if fail to install binary on linux": {
			linuxVersion: "Linux ip-172-31-35-135.us-west-2.compute.internal 4.14.203-156.332.amzn2.x86_64 #1 SMP Fri Oct 30 19:19:33 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("cat", []string{"/etc/os-release"}, gomock.Any()).DoAndReturn(
					func(name string, args []string, opts ...CmdOption) error {
						cmd := &osexec.Cmd{}
						for _, opt := range opts {
							opt(cmd)
						}
						cmd.Stdout.Write([]byte(`
"ID=centos"
"ID_LIKE="rhel fedora""`))
						return nil
					},
				)
				mockRunner.EXPECT().Run("sudo", []string{"yum", "install", "-y",
					filepath.Join(mockDir, "session-manager-plugin.rpm")}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to install binary on ubuntu": {
			linuxVersion: "Linux ip-172-31-0-242 5.4.0-1029-aws #30-Ubuntu SMP Tue Oct 20 10:06:38 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("cat", []string{"/etc/os-release"}, gomock.Any()).DoAndReturn(
					func(name string, args []string, opts ...CmdOption) error {
						cmd := &osexec.Cmd{}
						for _, opt := range opts {
							opt(cmd)
						}
						cmd.Stdout.Write([]byte(`
ID=ubuntu
ID_LIKE="debian"`))
						return nil
					},
				)
				mockRunner.EXPECT().Run("sudo", []string{"dpkg", "-i",
					filepath.Join(mockDir, "session-manager-plugin.deb")}).
					Return(mockError)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success on linux": {
			linuxVersion: "Linux ip-172-31-35-135.us-west-2.compute.internal 4.14.203-156.332.amzn2.x86_64 #1 SMP Fri Oct 30 19:19:33 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("cat", []string{"/etc/os-release"}, gomock.Any()).DoAndReturn(
					func(name string, args []string, opts ...CmdOption) error {
						cmd := &osexec.Cmd{}
						for _, opt := range opts {
							opt(cmd)
						}
						cmd.Stdout.Write([]byte(`
"ID=centos"
"ID_LIKE="rhel fedora""`))
						return nil
					},
				)
				mockRunner.EXPECT().Run("sudo", []string{"yum", "install", "-y",
					filepath.Join(mockDir, "session-manager-plugin.rpm")}).
					Return(nil)
			},
		},
		"success on ubuntu": {
			linuxVersion: "Linux ip-172-31-0-242 5.4.0-1029-aws #30-Ubuntu SMP Tue Oct 20 10:06:38 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("cat", []string{"/etc/os-release"}, gomock.Any()).DoAndReturn(
					func(name string, args []string, opts ...CmdOption) error {
						cmd := &osexec.Cmd{}
						for _, opt := range opts {
							opt(cmd)
						}
						cmd.Stdout.Write([]byte(`
"ID=pop"
"ID_LIKE="ubuntu debian""`))
						return nil
					},
				)
				mockRunner.EXPECT().Run("sudo", []string{"dpkg", "-i",
					filepath.Join(mockDir, "session-manager-plugin.deb")}).
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
				runner:                 mockRunner,
				tempDir:                mockDir,
				linuxDistVersionBuffer: *bytes.NewBufferString(tc.linuxVersion),
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
