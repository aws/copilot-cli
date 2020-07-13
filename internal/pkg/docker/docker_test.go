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
	mockImageTag := "mockImageTag"
	mockPath := "mockPath/to/mockDockerfile"

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		path       string
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			path: mockPath,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockImageTag), "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(mockError)
			},
			want: fmt.Errorf("building image: %w", mockError),
		},
		"happy path": {
			path: mockPath,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockImageTag), "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(nil)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			test.setupMocks(controller)
			s := Runner{
				runner: mockRunner,
			}

			got := s.Build(mockURI, mockImageTag, test.path)

			require.Equal(t, test.want, got)
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
	mockImageTag := "mockImageTag"

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockImageTag)}).Return(mockError)
			},
			want: fmt.Errorf("docker push: %w", mockError),
		},
		"happy path": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockImageTag)}).Return(nil)
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

			got := s.Push(mockURI, mockImageTag)

			require.Equal(t, test.want, got)
		})
	}
}

func TestBuildWithMultipleTags(t *testing.T) {
	mockURI := "mockURI"
	mockPath := "mockPath/to/mockDockerfile"

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		tags       []string
		path       string
		setupMocks func(controller *gomock.Controller)

		wantedError error
	}{
		"failed to run": {
			tags: []string{"tag1", "tag2"},
			path: mockPath,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, "tag1"), "-t", imageName(mockURI, "tag2"), "mockPath/to", "-f", "mockPath/to/mockDockerfile"}).Return(errors.New("error"))
			},
			wantedError: fmt.Errorf("building image: error"),
		},
		"no tag is provided": {
			tags: []string{},
			path: mockPath,
			setupMocks: func(controller *gomock.Controller) {},
			wantedError: errors.New("must provide at least one image tag"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			tc.setupMocks(controller)
			s := Runner{
				runner: mockRunner,
			}

			got := s.BuildWithMultipleTags(mockURI, tc.path,  tc.tags)

			require.EqualError(t, tc.wantedError, got.Error())
		})
	}
}

func TestPushWithMultipleTags(t *testing.T) {
	mockError := errors.New("mockError")
	mockURI := "mockURI"

	mockTag1 := "tag1"
	mockTag2 := "tag2"

	inTags := []string{mockTag1, mockTag2}

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"error running push": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag1)}).Return(mockError).Times(1)
				mockRunner.EXPECT().Run("docker", []string{"push", imageName(mockURI, mockTag2)}).Return(mockError).Times(1)
			},
			want: fmt.Errorf("docker push with multiple tags: push tag tag1: mockError; push tag tag2: mockError"),
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

			got := s.PushWithMultipleTags(mockURI, inTags)

			require.Equal(t, test.want, got)
		})
	}
}