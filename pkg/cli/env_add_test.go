package cli

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
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

func TestEnvAdd_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputOpts       AddEnvOpts
		wantedErrPrefix string
	}{
		"with no project name": {
			inputOpts: AddEnvOpts{
				EnvName: "coolapp",
			},
			wantedErrPrefix: "to add an environment either run the command in your workspace or provide a --project",
		},
		"with valid input": {
			inputOpts: AddEnvOpts{
				EnvName:     "coolapp",
				ProjectName: "coolproject",
			},
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

func TestEnvAdd_AddEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	var capturedArgument *archer.Environment
	defer ctrl.Finish()

	testCases := map[string]struct {
		addEnvOpts  AddEnvOpts
		expectedEnv archer.Environment
		mocking     func()
	}{
		"with a succesful call to add env": {
			addEnvOpts: AddEnvOpts{
				manager:     mockEnvStore,
				ProjectName: "project",
				EnvName:     "env",
				Production:  true,
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
				mockEnvStore.
					EXPECT().
					CreateEnvironment(gomock.Any()).
					Do(func(env *archer.Environment) {
						capturedArgument = env
					})

			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup mocks
			tc.mocking()

			tc.addEnvOpts.AddEnvironment()

			require.Equal(t, tc.expectedEnv, *capturedArgument)
		})
	}
}
