// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/exec/mocks"
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

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		path           string
		context        string
		additionalTags []string
		args           map[string]string
		target         string
		cacheFrom      []string
		setupMocks     func(controller *gomock.Controller)

		wantedError error
	}{
		"should error if the docker build command fails": {
			path:    mockPath,
			context: "",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(mockError)
			},
			wantedError: fmt.Errorf("building image: %w", mockError),
		},
		"should succeed in simple case with no context": {
			path:    mockPath,
			context: "",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", "mockURI:tag1", "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"context differs from path": {
			path:    mockPath,
			context: mockContext,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", mockURI + ":" + mockTag1, "mockPath", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"behaves the same if context is DF dir": {
			path:    mockPath,
			context: "mockPath/to",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", mockURI + ":" + mockTag1, "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"success with additional tags": {
			path:           mockPath,
			additionalTags: []string{mockTag2, mockTag3},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag2,
					"-t", mockURI + ":" + mockTag3,
					"-t", mockURI + ":" + mockTag1,
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
				mockRunner = mocks.NewMockrunner(c)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
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
				mockRunner = mocks.NewMockrunner(c)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", mockURI + ":" + mockTag1,
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
				Context:        tc.context,
				Dockerfile:     tc.path,
				URI:            mockURI,
				ImageTag:       mockTag1,
				AdditionalTags: tc.additionalTags,
				Args:           tc.args,
				Target:         tc.target,
				CacheFrom:      tc.cacheFrom,
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

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"login", "-u", mockUsername, "--password-stdin", mockURI}, gomock.Any()).Return(mockError)
			},
			want: fmt.Errorf("authenticate to ECR: %w", mockError),
		},
		"happy path": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

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
	mockError := errors.New("mockError")

	mockURI := "mockURI"

	mockTag1 := "tag1"
	mockTag2 := "tag2"

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"error running push": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", mockURI + ":" + mockTag1}).Return(mockError).Times(1)
				mockRunner.EXPECT().Run("docker", []string{"push", mockURI + ":" + mockTag2}).Times(0)
			},
			want: fmt.Errorf("docker push %s: %w", mockURI+":"+mockTag1, mockError),
		},
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", mockURI + ":" + mockTag1}).Return(nil)
				mockRunner.EXPECT().Run("docker", []string{"push", mockURI + ":" + mockTag2}).Return(nil)
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

			got := s.Push(mockURI, mockTag2, mockTag1)

			require.Equal(t, test.want, got)
		})
	}
}

func TestDockerCommand_IsDockerEngineRunning(t *testing.T) {
	mockError := errors.New("some error")
	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)
		inBuffer   *bytes.Buffer

		wantedMsg string
		wantedErr error
	}{
		"error running docker info": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(mockError)
			},

			wantedErr: fmt.Errorf("get docker info: some error"),
		},
		"return when docker is not installed": {
			inBuffer: bytes.NewBufferString("docker: command not found"),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(nil)
			},

			wantedMsg: "docker: command not found",
		},
		"return when docker engine is not started": {
			inBuffer: bytes.NewBufferString(`'{"ServerErrors":["Cannot connect to the Docker daemon at unix:///var/run/docker.sock.", "Is the docker daemon running?"]}'`),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"info", "-f", "'{{json .}}'"}, gomock.Any()).Return(nil)
			},

			wantedMsg: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock.\nIs the docker daemon running?",
		},
		"success": {
			inBuffer: bytes.NewBufferString(`'{"ID":"A2VY:4WTA:HDKK:UR76:SD2I:EQYZ:GCED:H4GT:6O7X:P72W:LCUP:ZQJD","Containers":15}'
`),
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
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

			msg, err := s.IsDockerEngineRunning()
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedMsg, msg)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}
