// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	cli_mocks "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestEnvAdd_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputEnv     string
		inputProject string
		input        func(c *expect.Console)

		wantedEnv     string
		wantedProject string
	}{
		"with no flags set": {
			input: func(c *expect.Console) {
				c.ExpectString("What is your project's name?")
				c.SendLine("project")
				c.ExpectString("What is your environment's name?")
				c.SendLine("env")
				c.ExpectEOF()
			},

			wantedEnv:     "env",
			wantedProject: "project",
		},
		"with env flags set": {
			input: func(c *expect.Console) {
				c.ExpectString("What is your project's name?")
				c.SendLine("project")
				c.ExpectEOF()
			},
			inputEnv:      "env",
			wantedEnv:     "env",
			wantedProject: "project",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			addEnv := &AddEnvOpts{
				EnvName:     tc.inputEnv,
				ProjectName: tc.inputProject,
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
			err := addEnv.Ask()

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, addEnv.ProjectName, "expected project names to match")
			require.Equal(t, tc.wantedEnv, addEnv.EnvName, "expected environment names to match")
		})
	}
}

func TestEnvAdd_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockProjStore := mocks.NewMockProjectStore(ctrl)
	mockDeployer := mocks.NewMockEnvironmentDeployer(ctrl)
	mockSpinner := cli_mocks.NewMockspinner(ctrl)
	var capturedArgument *archer.Environment
	defer ctrl.Finish()

	testCases := map[string]struct {
		addEnvOpts  AddEnvOpts
		expectedEnv archer.Environment
		expectedErr error
		mocking     func()
	}{
		"with a succesful call to add env": {
			addEnvOpts: AddEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjStore,
				deployer:      mockDeployer,
				ProjectName:   "project",
				EnvName:       "env",
				Production:    true,
				spinner:       mockSpinner,
			},
			expectedEnv: archer.Environment{
				Name:    "env",
				Project: "project",
				//TODO update these to real values
				AccountID:          "1234",
				Region:             "1234",
				Prod:               true,
				PublicLoadBalancer: true,
			},
			mocking: func() {
				gomock.InOrder(
					mockProjStore.
						EXPECT().
						GetProject(gomock.Any()).
						Return(&archer.Project{}, nil),
					mockSpinner.EXPECT().Start(gomock.Eq("Preparing deployment...")),
					mockDeployer.EXPECT().DeployEnvironment(gomock.Any()),
					mockSpinner.EXPECT().Stop(gomock.Eq("Done!")),
					mockSpinner.EXPECT().Start(gomock.Eq("Deploying env...")),
					// TODO: Assert Wait is called with stack name returned by DeployEnvironment.
					mockDeployer.EXPECT().WaitForEnvironmentCreation(gomock.Any()),
					mockEnvStore.
						EXPECT().
						CreateEnvironment(gomock.Any()).
						Do(func(env *archer.Environment) {
							capturedArgument = env
						}),
					mockSpinner.EXPECT().Stop(gomock.Eq("Done!")),
				)
			},
		},
		"with a invalid project": {
			expectedErr: mockError,
			addEnvOpts: AddEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjStore,
				deployer:      mockDeployer,
				ProjectName:   "project",
				EnvName:       "env",
				Production:    true,
				spinner:       mockSpinner,
			},
			expectedEnv: archer.Environment{
				Name:    "env",
				Project: "project",
				//TODO update these to real values
				AccountID: "1234",
				Region:    "1234",
				Prod:      true,
			},
			mocking: func() {
				mockProjStore.
					EXPECT().
					GetProject(gomock.Any()).
					Return(nil, mockError)
				mockEnvStore.
					EXPECT().
					CreateEnvironment(gomock.Any()).
					Times(0)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup mocks
			tc.mocking()

			err := tc.addEnvOpts.Execute()
			if tc.expectedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedEnv, *capturedArgument)
			} else {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}
