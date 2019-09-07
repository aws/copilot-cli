package cli

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestInit_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject string
		inputApp     string
		input        func(c *expect.Console)

		wantedProject string
		wantedApp     string
	}{
		"with no flags set": {
			inputProject: "",
			inputApp:     "",
			input: func(c *expect.Console) {
				c.ExpectString("What is your project's name?")
				c.SendLine("heartbeat")
				c.ExpectString("What is your application's name?")
				c.SendLine("api")
				c.ExpectEOF()
			},

			wantedProject: "heartbeat",
			wantedApp:     "api",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			app := &InitAppOpts{
				Project: tc.inputProject,
				Name:    tc.inputApp,
				Prompt: terminal.Stdio{
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
			err := app.Ask()

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, app.Project, "expected project names to match")
			require.Equal(t, tc.wantedApp, app.Name, "expected app names to match")
		})
	}
}

func TestInit_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputOpts       InitAppOpts
		wantedErrPrefix string
	}{
		"with valid project and app": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
		},
		"with invalid project name": {
			inputOpts: InitAppOpts{
				Name:    "coolapp",
				Project: "!!!!",
			},
			wantedErrPrefix: "project name invalid",
		},
		"with invalid app name": {
			inputOpts: InitAppOpts{
				Name:    "!!!",
				Project: "coolproject",
			},
			wantedErrPrefix: "application name invalid",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.inputOpts.Validate()
			if tc.wantedErrPrefix != "" {
				require.Regexp(t, "^"+tc.wantedErrPrefix+".*", err.Error())
			} else {
				require.NoError(t, err, "There shouldn't have been an error")
			}
		})
	}
}

func TestInit_InitApp(t *testing.T) {
	t.Skip("Skipping init, not yet implemented")
}
