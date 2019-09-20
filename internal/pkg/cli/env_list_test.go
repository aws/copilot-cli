// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestEnvList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts    ListEnvOpts
		output      func(c *expect.Console) bool
		mocking     func()
		expectedErr error
	}{
		"with envs": {
			output: func(c *expect.Console) bool {
				c.ExpectString("test")
				c.ExpectString("test2")
				return true
			},
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						&archer.Environment{Name: "test"},
						&archer.Environment{Name: "test2"},
					}, nil)

			},
		},
		"with invalid project name": {
			expectedErr: mockError,
			output: func(c *expect.Console) bool {
				return true
			},
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(nil, mockError)

				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			output: func(c *expect.Console) bool {
				return true
			},
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)

				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return(nil, mockError)
			},
		},
		"with production envs": {
			output: func(c *expect.Console) bool {
				c.ExpectString("test")
				c.ExpectString("test2 (prod)")
				return true
			},
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						&archer.Environment{Name: "test"},
						&archer.Environment{Name: "test2", Prod: true},
					}, nil)

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			// Prepare mocks
			tc.mocking()

			// Set up fake terminal
			tc.listOpts.prompt = terminal.Stdio{
				In:  mockTerminal.Tty(),
				Out: mockTerminal.Tty(),
				Err: mockTerminal.Tty(),
			}

			// Write inputs to the terminal
			done := make(chan bool)
			go func() { done <- tc.output(mockTerminal) }()

			// WHEN
			err := tc.listOpts.Execute()
			require.True(t, <-done, "We should print to the terminal")
			if tc.expectedErr != nil {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
			// Cleanup our terminals
			mockTerminal.Tty().Close()
			mockTerminal.Close()
		})
	}
}

func TestEnvList_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputEnv     string
		inputProject string
		input        func(c *expect.Console)

		wantedProject string
	}{
		"with no flags set": {
			input: func(c *expect.Console) {
				c.ExpectString("Which project's environments would you like to list?")
				c.SendLine("project")
				c.ExpectEOF()
			},

			wantedProject: "project",
		},
		"with env flags set": {
			input: func(c *expect.Console) {
				c.ExpectEOF()
			},
			inputProject:  "project",
			wantedProject: "project",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			listEnvs := &ListEnvOpts{
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
			err := listEnvs.Ask()

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listEnvs.ProjectName, "expected project names to match")
		})
	}
}
