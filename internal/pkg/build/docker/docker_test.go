// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/docker/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	mockError := errors.New("mockError")

	mockURI := "mockURI"
	mockImageTag := "mockImageTag"
	mockPath := "mockPath"

	var mockRunner *mocks.Mockrunner

	tests := map[string]struct {
		setupMocks func(controller *gomock.Controller)

		want error
	}{
		"wrap error returned from Run()": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockImageTag), mockPath}).Return(mockError)
			},
			want: fmt.Errorf("building image: %w", mockError),
		},
		"happy path": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)

				mockRunner.EXPECT().Run("docker", []string{"build", "-t", imageName(mockURI, mockImageTag), mockPath}).Return(nil)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			test.setupMocks(controller)
			s := Service{
				runner: mockRunner,
			}

			got := s.Build(mockURI, mockImageTag, mockPath)

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
			s := Service{
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
			s := Service{
				runner: mockRunner,
			}

			got := s.Push(mockURI, mockImageTag)

			require.Equal(t, test.want, got)
		})
	}
}

func TestParse(t *testing.T) {

	tests := map[string]struct {
		mockerfile string
		want       dockerfile
	}{
		"dockerfile with single expose": {
			mockerfile: `
EXPOSE 80
ARG myFunVar=50`,
			want: dockerfile{
				ambiguous:     false,
				exposedPorts:  []uint16{80},
				exposedTokens: []string{"80"},
				tokens:        map[string]string{"myFunVar": "50"},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			r := strings.NewReader(test.mockerfile)
			scanner := bufio.NewScanner(r)
			df := newDockerfile()
			for scanner.Scan() {
				res := strings
			}

		})
	}
}
