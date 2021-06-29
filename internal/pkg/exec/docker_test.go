// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDockerCommand_Build(t *testing.T) {
	mockError := errors.New("mockError")

	mockURI := "mockURI"
	mockPath := "mockPath/to/mockDockerfile"
	mockContext := "mockPath"

	mockTag1 := "tag1"
	mockTag2 := "tag2"
	mockTag3 := "tag3"

	var mockRunner *Mockrunner

	tests := map[string]struct {
		path       string
		context    string
		tags       []string
		args       map[string]string
		target     string
		cacheFrom  []string
		setupMocks func(controller *gomock.Controller)

		wantedError error
	}{
		"should error if the docker build command fails": {
			path:    mockPath,
			context: "",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"-t", mockURI + ":" + mockTag1,
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(mockError)
			},
			wantedError: fmt.Errorf("building image: %w", mockError),
		},
		"should succeed in simple case with no context": {
			path:    mockPath,
			context: "",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"-t", "mockURI:tag1", "mockPath/to",
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"context differs from path": {
			path:    mockPath,
			context: mockContext,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"mockPath",
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"behaves the same if context is DF dir": {
			path:    mockPath,
			context: "mockPath/to",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"-t", mockURI + ":" + mockTag1,
					"mockPath/to",
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"success with additional tags": {
			path: mockPath,
			tags: []string{mockTag1, mockTag2, mockTag3},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"-t", mockURI + ":" + mockTag1,
					"-t", mockURI + ":" + mockTag2,
					"-t", mockURI + ":" + mockTag3,
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"success with build args": {
			path: mockPath,
			args: map[string]string{
				"GOPROXY": "direct",
				"key":     "value",
				"abc":     "def",
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"--build-arg", "GOPROXY=direct",
					"--build-arg", "abc=def",
					"--build-arg", "key=value",
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"runs with cache_from and target fields": {
			path:      mockPath,
			target:    "foobar",
			cacheFrom: []string{"foo/bar:latest", "foo/bar/baz:1.2.3"},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI,
					"--cache-from", "foo/bar:latest",
					"--cache-from", "foo/bar/baz:1.2.3",
					"--target", "foobar",
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := DockerCommand{
				runner: mockRunner,
			}
			buildInput := BuildArguments{
				Context:    tc.context,
				Dockerfile: tc.path,
				URI:        mockURI,
				Args:       tc.args,
				Target:     tc.target,
				CacheFrom:  tc.cacheFrom,
				Tags:       tc.tags,
			}
			got := s.Build(&buildInput)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, got.Error())
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestDockerCommand_Login(t *testing.T) {
	mockError := errors.New("mockError")

	mockURI := "mockURI"
	mockUsername := "mockUsername"
	mockPassword := "mockPassword"

	var mockRunner *Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"login", "-u", mockUsername, "--password-stdin", mockURI}, gomock.Any()).Return(mockError)
			},
			want: fmt.Errorf("authenticate to ECR: %w", mockError),
		},
		"happy path": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"login", "-u", mockUsername, "--password-stdin", mockURI}, gomock.Any()).Return(nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			test.setupMocks(controller)
			s := DockerCommand{
				runner: mockRunner,
			}

			got := s.Login(mockURI, mockUsername, mockPassword)

			require.Equal(t, test.want, got)
		})
	}
}

func TestDockerCommand_Push(t *testing.T) {
	t.Run("pushes an image with multiple tags and returns its digest", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockrunner(ctrl)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app"}).Return(nil)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:g123bfc"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app"}, gomock.Any()).
			Do(func(_ string, _ []string, opt CmdOption) {
				cmd := &exec.Cmd{}
				opt(cmd)
				_, _ = cmd.Stdout.Write([]byte("\"aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app@sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807\"\n"))
			}).Return(nil)

		// WHEN
		cmd := DockerCommand{
			runner: m,
		}
		digest, err := cmd.Push("aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app", "g123bfc")

		// THEN
		require.NoError(t, err)
		require.Equal(t, "sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807", digest)
	})
	t.Run("returns a wrapped error on failed push to ecr", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockrunner(ctrl)
		m.EXPECT().Run(gomock.Any(), gomock.Any()).Return(errors.New("some error"))

		// WHEN
		cmd := DockerCommand{
			runner: m,
		}
		_, err := cmd.Push("uri")

		// THEN
		require.EqualError(t, err, "docker push uri: some error")
	})
	t.Run("returns a wrapped error on failure to retrieve image digest", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockrunner(ctrl)
		m.EXPECT().Run("docker", []string{"push", "uri"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "uri"}, gomock.Any()).Return(errors.New("some error"))

		// WHEN
		cmd := DockerCommand{
			runner: m,
		}
		_, err := cmd.Push("uri")

		// THEN
		require.EqualError(t, err, "inspect image digest for uri: some error")
	})
	t.Run("returns an error if the repo digest cannot be parsed for the pushed image", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockrunner(ctrl)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app"}).Return(nil)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:g123bfc"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app"}, gomock.Any()).
			Do(func(_ string, _ []string, opt CmdOption) {
				cmd := &exec.Cmd{}
				opt(cmd)
				_, _ = cmd.Stdout.Write([]byte(""))
			}).Return(nil)

		// WHEN
		cmd := DockerCommand{
			runner: m,
		}
		_, err := cmd.Push("aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app", "g123bfc")

		// THEN
		require.EqualError(t, err, "parse the digest from the repo digest ''")
	})
}

func TestDockerCommand_CheckDockerEngineRunning(t *testing.T) {
	mockError := errors.New("some error")
	var mockRunner *Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)
		inBuffer   *bytes.Buffer

		wantedErr error
	}{
		"error running docker info": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(mockError)
			},

			wantedErr: fmt.Errorf("get docker info: some error"),
		},
		"return when docker engine is not started": {
			inBuffer: bytes.NewBufferString(`'{"ServerErrors":["Cannot connect to the Docker daemon at unix:///var/run/docker.sock.", "Is the docker daemon running?"]}'`),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(nil)
			},

			wantedErr: &ErrDockerDaemonNotResponsive{
				msg: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock.\nIs the docker daemon running?",
			},
		},
		"success": {
			inBuffer: bytes.NewBufferString(`'{"ID":"A2VY:4WTA:HDKK:UR76:SD2I:EQYZ:GCED:H4GT:6O7X:P72W:LCUP:ZQJD","Containers":15}'
`),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := DockerCommand{
				runner: mockRunner,
				buf:    tc.inBuffer,
			}

			err := s.CheckDockerEngineRunning()
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestDockerCommand_GetPlatform(t *testing.T) {
	mockError := errors.New("some error")
	mockOS := "operatingSystem"
	mockArch := "archie"
	var mockRunner *Mockrunner

	tests := map[string]struct {
		setupMocks   func(controller *gomock.Controller)
		inOSBuffer   *bytes.Buffer
		inArchBuffer *bytes.Buffer
		wantedOS     string
		wantedArch   string

		wantedErr error
	}{
		"error running 'docker version' for os": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"version", "-f", "'{{.Server.Os}}'"}, gomock.Any()).Return(mockError)
			},
			wantedErr: fmt.Errorf("get docker os: some error"),
		},
		"error running 'docker version' for arch": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"version", "-f", "'{{.Server.Os}}'"}, gomock.Any()).Return(nil)
				mockRunner.EXPECT().Run("docker", []string{"version", "-f", "'{{.Server.Arch}}'"}, gomock.Any()).Return(mockError)
			},
			wantedErr: fmt.Errorf("get docker architecture: some error"),
		},
		"success": {
			inOSBuffer:   bytes.NewBufferString(`linux`),
			inArchBuffer: bytes.NewBufferString(`amd64`),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"version", "-f", "'{{.Server.Os}}'"}, gomock.Any()).Return(nil)
				mockRunner.EXPECT().Run("docker", []string{"version", "-f", "'{{.Server.Arch}}'"}, gomock.Any()).Return(nil)
			},
			wantedOS:   mockOS,
			wantedArch: mockArch,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := DockerCommand{
				runner: mockRunner,
				buf:    tc.inOSBuffer,
			}

			os, arch, err := s.GetPlatform()
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
				if tc.wantedOS != "" {
					require.Equal(t, tc.wantedOS, os)
				}
				if tc.wantedArch != "" {
					require.Equal(t, tc.wantedArch, arch)
				}
			}
		})
	}
}

func TestIsEcrCredentialHelperEnabled(t *testing.T) {
	var mockRunner *Mockrunner
	workspace := "test/copilot/.docker"
	registry := "dummyaccountid.dkr.ecr.region.amazonaws.com"
	uri := fmt.Sprintf("%s/ui/app", registry)

	testCases := map[string]struct {
		setupMocks     func(controller *gomock.Controller)
		inBuffer       *bytes.Buffer
		mockFileSystem func(fs afero.Fs)
		postExec       func(fs afero.Fs)
		isEcrRepo      bool
	}{
		"ecr-login check global level": {
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll(workspace, 0755)
				afero.WriteFile(fs, filepath.Join(workspace, "config.json"), []byte(fmt.Sprintf("{\"credsStore\":\"%s\"}", credStoreECRLogin)), 0644)
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
			},
			postExec: func(fs afero.Fs) {
				fs.RemoveAll(workspace)
			},
			isEcrRepo: true,
		},
		"ecr-login check registry level": {
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll(workspace, 0755)
				afero.WriteFile(fs, filepath.Join(workspace, "config.json"), []byte(fmt.Sprintf("{\"credhelpers\":{\"%s\": \"%s\"}}", registry, credStoreECRLogin)), 0644)
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
			},
			postExec: func(fs afero.Fs) {
				fs.RemoveAll(workspace)
			},
			isEcrRepo: true,
		},
		"default login check registry level": {
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll(workspace, 0755)
				afero.WriteFile(fs, filepath.Join(workspace, "config.json"), []byte(fmt.Sprintf("{\"credhelpers\":{\"%s\": \"%s\"}}", registry, "desktop")), 0644)
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
			},
			postExec: func(fs afero.Fs) {
				fs.RemoveAll(workspace)
			},
			isEcrRepo: false,
		},
		"no file check": {
			mockFileSystem: func(fs afero.Fs) {
				fs.MkdirAll(workspace, 0755)
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = NewMockrunner(c)
			},
			postExec: func(fs afero.Fs) {
				fs.RemoveAll(workspace)
			},
			isEcrRepo: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewOsFs()

			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			tc.mockFileSystem(fs)
			s := DockerCommand{
				runner:   mockRunner,
				buf:      tc.inBuffer,
				homePath: "test/copilot",
			}

			credStore := s.IsEcrCredentialHelperEnabled(uri)
			tc.postExec(fs)

			require.Equal(t, tc.isEcrRepo, credStore)
		})
	}
}
