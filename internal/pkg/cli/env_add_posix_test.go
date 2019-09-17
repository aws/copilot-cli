// +build !windows

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
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
			mockTerminal, _, err := vt10x.NewVT10XConsole()
			require.NoError(t, err)
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
			err = addEnv.Ask()

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
