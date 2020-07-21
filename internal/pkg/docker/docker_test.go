// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package docker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/docker/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
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
		setupMocks     func(controller *gomock.Controller)

		wantedError error
	}{
		"should error if the docker build command fails": {
			path:    mockPath,
			context: "",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", imageName(mockURI, mockTag1),
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

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockTag1), "mockPath", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"behaves the same if context is DF dir": {
			path:    mockPath,
			context: "mockPath/to",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockTag1), "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},

		"success with additional tags": {
			path:           mockPath,
			additionalTags: []string{mockTag2, mockTag3},
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", imageName(mockURI, mockTag2),
					"-t", imageName(mockURI, mockTag3),
					"-t", imageName(mockURI, mockTag1),
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
		"success with build args": {
			path: mockPath,
			args: map[string]string{
				"key":  "value",
				"key2": "value2",
			},
			setupMocks: func(c *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(c)
				mockRunner.EXPECT().Run("docker", []string{"build",
					"-t", imageName(mockURI, mockTag1),
					"--build-arg", "key=value",
					"--build-arg", "key2=value2",
					"mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := Runner{
				runner: mockRunner,
			}
			buildInput := BuildArguments{
				Context:        tc.context,
				Dockerfile:     tc.path,
				URI:            mockURI,
				ImageTag:       mockTag1,
				AdditionalTags: tc.additionalTags,
				Args:           tc.args,
			}
			got := s.Build(buildInput)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, got.Error())
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestLogin(t *testing.T) {
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
			s := Runner{
				runner: mockRunner,
			}

			got := s.Login(mockURI, mockUsername, mockPassword)

			require.Equal(t, test.want, got)
		})
	}
}

func TestPush(t *testing.T) {
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

				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag1)}).Return(mockError).Times(1)
				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag2)}).Times(0)
			},
			want: fmt.Errorf("docker push %s: %w", imageName(mockURI, mockTag1), mockError),
		},
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag1)}).Return(nil)
				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag2)}).Return(nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			test.setupMocks(controller)
			s := Runner{
				runner: mockRunner,
			}

			got := s.Push(mockURI, mockTag2, mockTag1)

			require.Equal(t, test.want, got)
		})
	}
}
