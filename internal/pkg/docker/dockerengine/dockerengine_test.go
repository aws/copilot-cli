// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerengine

import (
	"bytes"
	"errors"
	"fmt"
	osexec "os/exec"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
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
	mockContainerName := "mockWkld"

	var mockCmd *MockCmd

	tests := map[string]struct {
		path       string
		context    string
		tags       []string
		args       map[string]string
		target     string
		cacheFrom  []string
		envVars    map[string]string
		labels     map[string]string
		setupMocks func(controller *gomock.Controller)

		wantedError error
	}{
		"should error tags are not specified": {
			path:    mockPath,
			context: "",
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
			},
			wantedError: fmt.Errorf("tags to reference an image should not be empty for building and pushing into the ECR repository %s", mockURI),
		},
		"should error if the docker build command fails": {
			path:    mockPath,
			context: "",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
					filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(mockError)
			},
			wantedError: fmt.Errorf("building image: %w", mockError),
		},
		"should succeed in simple case with no context": {
			path:    mockPath,
			context: "",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)

				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", "mockURI:tag1", filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"should display quiet progress updates when in a CI environment": {
			path:    mockPath,
			tags:    []string{mockTag1},
			context: "",
			envVars: map[string]string{
				"CI": "true",
			},
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)

				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", fmt.Sprintf("%s:%s", mockURI, mockTag1),
					"--progress", "plain",
					filepath.FromSlash("mockPath/to"), "-f", "mockPath/to/mockDockerfile"}).
					Return(nil)
			},
		},
		"context differs from path": {
			path:    mockPath,
			tags:    []string{mockTag1},
			context: mockContext,
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", fmt.Sprintf("%s:%s", mockURI, mockTag1),
					"mockPath",
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"behaves the same if context is DF dir": {
			path:    mockPath,
			context: "mockPath/to",
			tags:    []string{mockTag1},
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)

				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
					"mockPath/to",
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"success with additional tags": {
			path: mockPath,
			tags: []string{mockTag1, mockTag2, mockTag3},
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
					"-t", mockURI + ":" + mockTag2,
					"-t", mockURI + ":" + mockTag3,
					filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"success with build args": {
			path: mockPath,
			tags: []string{"latest"},
			args: map[string]string{
				"GOPROXY": "direct",
				"key":     "value",
				"abc":     "def",
			},
			setupMocks: func(c *gomock.Controller) {
				mockCmd = NewMockCmd(c)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", fmt.Sprintf("%s:%s", mockURI, "latest"),
					"--build-arg", "GOPROXY=direct",
					"--build-arg", "abc=def",
					"--build-arg", "key=value",
					filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"success with labels": {
			path: mockPath,
			tags: []string{"latest"},
			labels: map[string]string{
				"com.aws.copilot.image.version":        "v1.26.0",
				"com.aws.copilot.image.builder":        "copilot-cli",
				"com.aws.copilot.image.container.name": mockContainerName,
			},
			setupMocks: func(c *gomock.Controller) {
				mockCmd = NewMockCmd(c)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", fmt.Sprintf("%s:%s", mockURI, "latest"),
					"--label", "com.aws.copilot.image.version=v1.26.0",
					"--label", "com.aws.copilot.image.builder=copilot-cli",
					"--label", "com.aws.copilot.image.container.name=mockWkld",
					filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"runs with cache_from and target fields": {
			path:      mockPath,
			tags:      []string{"latest"},
			target:    "foobar",
			cacheFrom: []string{"foo/bar:latest", "foo/bar/baz:1.2.3"},
			setupMocks: func(c *gomock.Controller) {
				mockCmd = NewMockCmd(c)
				mockCmd.EXPECT().Run("docker", []string{"build",
					"-t", fmt.Sprintf("%s:%s", mockURI, "latest"),
					"--cache-from", "foo/bar:latest",
					"--cache-from", "foo/bar/baz:1.2.3",
					"--target", "foobar",
					filepath.FromSlash("mockPath/to"),
					"-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := CmdClient{
				runner: mockCmd,
				lookupEnv: func(key string) (string, bool) {
					if val, ok := tc.envVars[key]; ok {
						return val, true
					}
					return "", false
				},
			}
			buildInput := BuildArguments{
				Context:    tc.context,
				Dockerfile: tc.path,
				URI:        mockURI,
				Args:       tc.args,
				Target:     tc.target,
				CacheFrom:  tc.cacheFrom,
				Tags:       tc.tags,
				Labels:     tc.labels,
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

	var mockCmd *MockCmd

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)

				mockCmd.EXPECT().Run("docker", []string{"login", "-u", mockUsername, "--password-stdin", mockURI}, gomock.Any()).Return(mockError)
			},
			want: fmt.Errorf("authenticate to ECR: %w", mockError),
		},
		"happy path": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)

				mockCmd.EXPECT().Run("docker", []string{"login", "-u", mockUsername, "--password-stdin", mockURI}, gomock.Any()).Return(nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			test.setupMocks(controller)
			s := CmdClient{
				runner: mockCmd,
			}

			got := s.Login(mockURI, mockUsername, mockPassword)

			require.Equal(t, test.want, got)
		})
	}
}

func TestDockerCommand_Push(t *testing.T) {
	emptyLookupEnv := func(key string) (string, bool) {
		return "", false
	}
	t.Run("pushes an image with multiple tags and returns its digest", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockCmd(ctrl)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:latest"}).Return(nil)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:g123bfc"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:latest"}, gomock.Any()).
			Do(func(_ string, _ []string, opt exec.CmdOption) {
				cmd := &osexec.Cmd{}
				opt(cmd)
				_, _ = cmd.Stdout.Write([]byte("\"aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app@sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807\"\n"))
			}).Return(nil)

		// WHEN
		cmd := CmdClient{
			runner:    m,
			lookupEnv: emptyLookupEnv,
		}
		digest, err := cmd.Push("aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app", "latest", "g123bfc")

		// THEN
		require.NoError(t, err)
		require.Equal(t, "sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807", digest)
	})
	t.Run("should display quiet progress updates when in a CI environment", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockCmd(ctrl)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:latest", "--quiet"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:latest"}, gomock.Any()).
			Do(func(_ string, _ []string, opt exec.CmdOption) {
				cmd := &osexec.Cmd{}
				opt(cmd)
				_, _ = cmd.Stdout.Write([]byte("\"aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app@sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807\"\n"))
			}).Return(nil)

		// WHEN
		cmd := CmdClient{
			runner: m,
			lookupEnv: func(key string) (string, bool) {
				if key == "CI" {
					return "true", true
				}
				return "", false
			},
		}
		digest, err := cmd.Push("aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app", "latest")

		// THEN
		require.NoError(t, err)
		require.Equal(t, "sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807", digest)
	})
	t.Run("returns a wrapped error on failed push to ecr", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockCmd(ctrl)
		m.EXPECT().Run(gomock.Any(), gomock.Any()).Return(errors.New("some error"))

		// WHEN
		cmd := CmdClient{
			runner:    m,
			lookupEnv: emptyLookupEnv,
		}
		_, err := cmd.Push("uri", "latest")

		// THEN
		require.EqualError(t, err, "docker push uri:latest: some error")
	})
	t.Run("returns a wrapped error on failure to retrieve image digest", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockCmd(ctrl)
		m.EXPECT().Run("docker", []string{"push", "uri:latest"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "uri:latest"}, gomock.Any()).Return(errors.New("some error"))

		// WHEN
		cmd := CmdClient{
			runner:    m,
			lookupEnv: emptyLookupEnv,
		}
		_, err := cmd.Push("uri", "latest")

		// THEN
		require.EqualError(t, err, "inspect image digest for uri: some error")
	})
	t.Run("returns an error if the repo digest cannot be parsed for the pushed image", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := NewMockCmd(ctrl)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:latest"}).Return(nil)
		m.EXPECT().Run("docker", []string{"push", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app:g123bfc"}).Return(nil)
		m.EXPECT().Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", "aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app"}, gomock.Any()).
			Do(func(_ string, _ []string, opt exec.CmdOption) {
				cmd := &osexec.Cmd{}
				opt(cmd)
				_, _ = cmd.Stdout.Write([]byte(""))
			}).Return(nil)

		// WHEN
		cmd := CmdClient{
			runner:    m,
			lookupEnv: emptyLookupEnv,
		}
		_, err := cmd.Push("aws_account_id.dkr.ecr.region.amazonaws.com/my-web-app", "latest", "g123bfc")

		// THEN
		require.EqualError(t, err, "parse the digest from the repo digest ''")
	})
}

func TestDockerCommand_CheckDockerEngineRunning(t *testing.T) {
	mockError := errors.New("some error")
	var mockCmd *MockCmd

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		wantedErr error
	}{
		"error running docker info": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(mockError)
			},

			wantedErr: fmt.Errorf("get docker info: some error"),
		},
		"return when docker engine is not started": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).
					Do(func(_ string, _ []string, opt exec.CmdOption) {
						cmd := &osexec.Cmd{}
						opt(cmd)
						_, _ = cmd.Stdout.Write([]byte(`'{"ServerErrors":["Cannot connect to the Docker daemon at unix:///var/run/docker.sock.", "Is the docker daemon running?"]}'`))
					}).Return(nil)
			},

			wantedErr: &ErrDockerDaemonNotResponsive{
				msg: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock.\nIs the docker daemon running?",
			},
		},
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).
					Do(func(_ string, _ []string, opt exec.CmdOption) {
						cmd := &osexec.Cmd{}
						opt(cmd)
						_, _ = cmd.Stdout.Write([]byte(`'{"ID":"A2VY:4WTA:HDKK:UR76:SD2I:EQYZ:GCED:H4GT:6O7X:P72W:LCUP:ZQJD","Containers":15}'
`))
					}).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := CmdClient{
				runner: mockCmd,
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
	var mockCmd *MockCmd

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)
		wantedOS   string
		wantedArch string

		wantedErr error
	}{
		"error running 'docker version'": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"version", "-f", "'{{json .Server}}'"}, gomock.Any()).Return(mockError)
			},
			wantedOS:   "",
			wantedArch: "",
			wantedErr:  fmt.Errorf("run docker version: some error"),
		},
		"successfully returns os and arch": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"version", "-f", "'{{json .Server}}'"}, gomock.Any()).
					Do(func(_ string, _ []string, opt exec.CmdOption) {
						cmd := &osexec.Cmd{}
						opt(cmd)
						_, _ = cmd.Stdout.Write([]byte("{\"Platform\":{\"Name\":\"Docker CmdClient - Community\"},\"Components\":[{\"Name\":\"CmdClient\",\"Version\":\"20.10.6\",\"Details\":{\"ApiVersion\":\"1.41\",\"Arch\":\"amd64\",\"BuildTime\":\"Fri Apr  9 22:44:56 2021\",\"Experimental\":\"false\",\"GitCommit\":\"8728dd2\",\"GoVersion\":\"go1.13.15\",\"KernelVersion\":\"5.10.25-linuxkit\",\"MinAPIVersion\":\"1.12\",\"Os\":\"linux\"}},{\"Name\":\"containerd\",\"Version\":\"1.4.4\",\"Details\":{\"GitCommit\":\"05f951a3781f4f2c1911b05e61c16e\"}},{\"Name\":\"runc\",\"Version\":\"1.0.0-rc93\",\"Details\":{\"GitCommit\":\"12644e614e25b05da6fd00cfe1903fdec\"}},{\"Name\":\"docker-init\",\"Version\":\"0.19.0\",\"Details\":{\"GitCommit\":\"de40ad0\"}}],\"Version\":\"20.10.6\",\"ApiVersion\":\"1.41\",\"MinAPIVersion\":\"1.12\",\"GitCommit\":\"8728dd2\",\"GoVersion\":\"go1.13.15\",\"Os\":\"linux\",\"Arch\":\"amd64\",\"KernelVersion\":\"5.10.25-linuxkit\",\"BuildTime\":\"2021-04-09T22:44:56.000000000+00:00\"}\n"))
					}).Return(nil)
			},
			wantedOS:   "linux",
			wantedArch: "amd64",
			wantedErr:  nil,
		},
		"successfully returns 'windows/amd64' if that's what's detected": {
			setupMocks: func(controller *gomock.Controller) {
				mockCmd = NewMockCmd(controller)
				mockCmd.EXPECT().Run("docker", []string{"version", "-f", "'{{json .Server}}'"}, gomock.Any()).
					Do(func(_ string, _ []string, opt exec.CmdOption) {
						cmd := &osexec.Cmd{}
						opt(cmd)
						_, _ = cmd.Stdout.Write([]byte("{\"Platform\":{\"Name\":\"Docker CmdClient - Community\"},\"Components\":[{\"Name\":\"CmdClient\",\"Version\":\"20.10.6\",\"Details\":{\"ApiVersion\":\"1.41\",\"Arch\":\"amd64\",\"BuildTime\":\"Fri Apr  9 22:44:56 2021\",\"Experimental\":\"false\",\"GitCommit\":\"8728dd2\",\"GoVersion\":\"go1.13.15\",\"KernelVersion\":\"5.10.25-linuxkit\",\"MinAPIVersion\":\"1.12\",\"Os\":\"linux\"}},{\"Name\":\"containerd\",\"Version\":\"1.4.4\",\"Details\":{\"GitCommit\":\"05f951a3781f4f2c1911b05e61c16e\"}},{\"Name\":\"runc\",\"Version\":\"1.0.0-rc93\",\"Details\":{\"GitCommit\":\"12644e614e25b05da6fd00cfe1903fdec\"}},{\"Name\":\"docker-init\",\"Version\":\"0.19.0\",\"Details\":{\"GitCommit\":\"de40ad0\"}}],\"Version\":\"20.10.6\",\"ApiVersion\":\"1.41\",\"MinAPIVersion\":\"1.12\",\"GitCommit\":\"8728dd2\",\"GoVersion\":\"go1.13.15\",\"Os\":\"windows\",\"Arch\":\"amd64\",\"KernelVersion\":\"5.10.25-linuxkit\",\"BuildTime\":\"2021-04-09T22:44:56.000000000+00:00\"}\n"))
					}).Return(nil)
			},
			wantedOS:   "windows",
			wantedArch: "amd64",
			wantedErr:  nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := CmdClient{
				runner: mockCmd,
			}

			os, arch, err := s.GetPlatform()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedOS, os)
				require.Equal(t, tc.wantedArch, arch)
				require.NoError(t, err)
			}
		})
	}
}

func TestIsEcrCredentialHelperEnabled(t *testing.T) {
	var mockCmd *MockCmd
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
				mockCmd = NewMockCmd(c)
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
				mockCmd = NewMockCmd(c)
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
				mockCmd = NewMockCmd(c)
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
				mockCmd = NewMockCmd(c)
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
			s := CmdClient{
				runner:   mockCmd,
				buf:      tc.inBuffer,
				homePath: "test/copilot",
			}

			credStore := s.IsEcrCredentialHelperEnabled(uri)
			tc.postExec(fs)

			require.Equal(t, tc.isEcrRepo, credStore)
		})
	}
}
