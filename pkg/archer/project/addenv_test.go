// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/env"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hinshun/vt10x"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type mockValidStore struct {
	projectName string
}

func (m *mockValidStore) List() ([]*Project, error) {
	return []*Project{
		{
			Name: m.projectName,
		},
	}, nil
}

type mockEmptyStore struct{}

func (m *mockEmptyStore) List() ([]*Project, error) {
	return []*Project{}, nil
}

type mockInvalidStore struct {
	err error
}

func (m *mockInvalidStore) List() ([]*Project, error) {
	return nil, m.err
}

func TestAddEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		mockStore     projectLister
		input         func(c *expect.Console) // Interactions with the terminal

		wantedProjectName string
		wantedEnvName     string
		wantedErr         error
	}{
		"flags provided": {
			inProjectName: "chicken",
			inEnvName:     "test",
			mockStore:     nil,
			input:         func(c *expect.Console) {},

			wantedProjectName: "chicken",
			wantedEnvName:     "test",
			wantedErr:         nil,
		},
		"select project name from the list": {
			inProjectName: "",
			inEnvName:     "test",
			mockStore:     &mockValidStore{projectName: "chicken"},
			input: func(c *expect.Console) {
				c.ExpectString("Which project do you want to create this environment in?")
				c.SendLine("") // Select the first option
				c.ExpectEOF()
			},

			wantedProjectName: "chicken",
			wantedEnvName:     "test",
			wantedErr:         nil,
		},
		"no existing projects yet": {
			inProjectName: "",
			inEnvName:     "test",
			mockStore:     &mockEmptyStore{},
			input:         func(c *expect.Console) {},

			wantedProjectName: "",
			wantedEnvName:     "test",
			wantedErr:         ErrNoExistingProjects,
		},
		"error while listing options": {
			inProjectName: "",
			inEnvName:     "",
			mockStore:     &mockInvalidStore{err: errors.New("some SSM error")},
			input:         func(c *expect.Console) {},

			wantedProjectName: "",
			wantedEnvName:     "",
			wantedErr:         errors.New("some SSM error"),
		},
		"enter environment name": {
			inProjectName: "chicken",
			inEnvName:     "",
			mockStore:     nil,
			input: func(c *expect.Console) {
				c.ExpectString("What is your environment's name?")
				c.SendLine("test")
				c.ExpectEOF()
			},

			wantedProjectName: "chicken",
			wantedEnvName:     "test",
			wantedErr:         nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			opts := &AddEnvOpts{
				ProjectName: tc.inProjectName,
				EnvName:     tc.inEnvName,
				store:       tc.mockStore,
				prompt: terminal.Stdio{
					In:  mockTerminal.Tty(),
					Out: mockTerminal.Tty(),
					Err: mockTerminal.Tty(),
				},
			}

			// Write inputs to the terminal
			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.input(mockTerminal)
			}()

			// WHEN
			err := opts.Ask()

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			require.Equal(t, opts.ProjectName, tc.wantedProjectName, "expected project names to be equal")
			require.Equal(t, opts.EnvName, tc.wantedEnvName, "expected env names to be equal")
			if tc.wantedErr == nil {
				require.NoError(t, err, "unexpected error")
			} else {
				require.EqualError(t, err, tc.wantedErr.Error(), "expected errors to be equal")
			}
		})
	}
}

type mockSSM struct {
	t *testing.T

	mockPutParameter        func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
}

func (m *mockSSM) PutParameter(in *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	return m.mockPutParameter(m.t, in)
}

func (m *mockSSM) GetParametersByPath(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	return m.mockGetParametersByPath(m.t, in)
}

func TestProject_AddEnv(t *testing.T) {
	testCases := map[string]struct {
		inProjectName  string
		inEnvName      string
		inEnvRegion    string
		inEnvAccountID string

		mockPutParameter func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)

		wantedError error
	}{
		"valid parameter": {
			inProjectName:  "chicken",
			inEnvName:      "test",
			inEnvRegion:    "us-west-2",
			inEnvAccountID: "1111",

			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, "/archer/chicken/environments/test", *param.Name)
				require.Equal(t, `{"name":"test","region":"us-west-2","accountID":"1111"}`, *param.Value)
				return nil, nil
			},
			wantedError: nil,
		},
		"parameter already exists": {
			inProjectName:  "chicken",
			inEnvName:      "test",
			inEnvRegion:    "us-west-2",
			inEnvAccountID: "1111",

			mockPutParameter: func(t *testing.T, param *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
				require.Equal(t, "/archer/chicken/environments/test", *param.Name)
				require.Equal(t, `{"name":"test","region":"us-west-2","accountID":"1111"}`, *param.Value)
				return nil, awserr.New(ssm.ErrCodeParameterAlreadyExists, "", nil)
			},
			wantedError: &ErrEnvAlreadyExists{Name: "test", Project: "chicken"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			p := &Project{
				Name: tc.inProjectName,
				c: &mockSSM{
					t:                t,
					mockPutParameter: tc.mockPutParameter,
				},
			}
			e := &env.Environment{
				Name:      tc.inEnvName,
				Region:    tc.inEnvRegion,
				AccountID: tc.inEnvAccountID,
			}

			// WHEN
			err := p.AddEnv(e)

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err, "unexpected error")
			} else {
				require.EqualError(t, err, tc.wantedError.Error(), "expected same error message")
			}
		})
	}
}
