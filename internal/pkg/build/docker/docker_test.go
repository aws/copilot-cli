package docker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/ecr"
	"github.com/stretchr/testify/require"
)

type mockRunnable struct {
	t *testing.T

	mockRun           func() error
	mockStandardInput func(t *testing.T, input string)
}

func (mr mockRunnable) run() error {
	return mr.mockRun()
}

func (mr mockRunnable) standardInput(input string) {
	mr.mockStandardInput(mr.t, input)
}

func TestBuild(t *testing.T) {
	mockError := errors.New("mockError")

	mockURI := "mockURI"
	mockImageTag := "mockImageTag"
	mockPath := "mockPath"

	tests := map[string]struct {
		commandName string
		args        []string

		mockRun func() error

		want error
	}{
		"wrap error returned from Run()": {
			commandName: "docker",
			args:        []string{"build", "-t", imageName(mockURI, mockImageTag), mockPath},
			mockRun: func() error {
				return mockError
			},
			want: fmt.Errorf("building image: %w", mockError),
		},
		"happy path": {
			commandName: "docker",
			args:        []string{"build", "-t", imageName(mockURI, mockImageTag), mockPath},
			mockRun: func() error {
				return nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := Service{
				createCommand: func(name string, args ...string) runnable {
					require.Equal(t, test.commandName, name)
					require.ElementsMatch(t, test.args, args)

					return mockRunnable{
						t:       t,
						mockRun: test.mockRun,
					}
				},
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

	mockAuth := ecr.Auth{
		Username: mockUsername,
		Password: mockPassword,
	}

	tests := map[string]struct {
		commandName string
		args        []string

		mockStandardInput func(t *testing.T, input string)
		mockRun           func() error

		want error
	}{
		"wrap error returned from Run()": {
			commandName: "docker",
			args:        []string{"login", "-u", mockUsername, "--password-stdin", mockURI},
			mockStandardInput: func(t *testing.T, input string) {
				t.Helper()

				require.Equal(t, mockPassword, input)
			},
			mockRun: func() error {
				return mockError
			},
			want: fmt.Errorf("authenticate to ECR: %w", mockError),
		},
		"happy path": {
			commandName: "docker",
			args:        []string{"login", "-u", mockUsername, "--password-stdin", mockURI},
			mockStandardInput: func(t *testing.T, input string) {
				t.Helper()

				require.Equal(t, mockPassword, input)
			},
			mockRun: func() error {
				return nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := Service{
				createCommand: func(name string, args ...string) runnable {
					require.Equal(t, test.commandName, name)
					require.ElementsMatch(t, test.args, args)

					return mockRunnable{
						t:                 t,
						mockRun:           test.mockRun,
						mockStandardInput: test.mockStandardInput,
					}
				},
			}

			got := s.Login(mockURI, mockAuth)

			require.Equal(t, test.want, got)
		})
	}
}

func TestPush(t *testing.T) {
	mockError := errors.New("mockError")

	mockURI := "mockURI"
	mockImageTag := "mockImageTag"

	tests := map[string]struct {
		commandName string
		args        []string

		mockRun func() error

		want error
	}{
		"wrap error returned from Run()": {
			commandName: "docker",
			args:        []string{"push", imageName(mockURI, mockImageTag)},
			mockRun: func() error {
				return mockError
			},
			want: fmt.Errorf("docker push: %w", mockError),
		},
		"happy path": {
			commandName: "docker",
			args:        []string{"push", imageName(mockURI, mockImageTag)},
			mockRun: func() error {
				return nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := Service{
				createCommand: func(name string, args ...string) runnable {
					require.Equal(t, test.commandName, name)
					require.ElementsMatch(t, test.args, args)

					return mockRunnable{
						t:       t,
						mockRun: test.mockRun,
					}
				},
			}

			got := s.Push(mockURI, mockImageTag)

			require.Equal(t, test.want, got)
		})
	}
}
