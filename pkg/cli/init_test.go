package cli

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/golang/mock/gomock"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestInit_Ask(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputProject     string
		inputApp         string
		inputType        string
		input            func(c *expect.Console)
		wantedProject    string
		wantedApp        string
		wantedType       string
		existingProjects []string
	}{
		"with no flags set and no projects": {
			input: func(c *expect.Console) {
				c.ExpectString("What is your project's name?")
				c.SendLine("heartbeat")
				c.ExpectString("What is your application's name?")
				c.SendLine("api")
				c.ExpectString("Which template would you like to use?")
				c.SendLine(string(terminal.KeyEnter))
				c.ExpectString("Would you like to set up a test environment")
				c.SendLine("n")
				c.ExpectEOF()
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with no flags set and existing projects": {
			existingProjects: []string{"heartbeat"},
			input: func(c *expect.Console) {
				c.ExpectString("Which project should we use?")
				c.SendLine(string(terminal.KeyEnter))
				c.ExpectString("What is your application's name?")
				c.SendLine("api")
				c.ExpectString("Which template would you like to use?")
				c.SendLine(string(terminal.KeyEnter))
				c.ExpectString("Would you like to set up a test environment")
				c.SendLine("n")
				c.ExpectEOF()
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with only project flag set": {
			inputProject: "heartbeat",
			input: func(c *expect.Console) {
				c.ExpectString("What is your application's name?")
				c.SendLine("api")
				c.ExpectString("Which template would you like to use?")
				c.SendLine(string(terminal.KeyEnter))
				c.ExpectString("Would you like to set up a test environment")
				c.SendLine("n")
				c.ExpectEOF()
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with project and app flag set": {
			inputProject: "heartbeat",
			inputApp:     "api",
			input: func(c *expect.Console) {
				c.ExpectString("Which template would you like to use?")
				c.SendLine(string(terminal.KeyEnter))
				c.ExpectString("Would you like to set up a test environment")
				c.SendLine("n")
				c.ExpectEOF()
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with project, app and template flag set": {
			inputProject: "heartbeat",
			inputApp:     "api",
			inputType:    "Load Balanced Web App",
			input: func(c *expect.Console) {
				c.ExpectString("Would you like to set up a test environment")
				c.SendLine("n")
				c.ExpectEOF()
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
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
				Type:    tc.inputType,
				prompt: terminal.Stdio{
					In:  mockTerminal.Tty(),
					Out: mockTerminal.Tty(),
					Err: mockTerminal.Tty(),
				},
				projStore:        mockProjectStore,
				existingProjects: tc.existingProjects,
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
			require.Equal(t, tc.wantedType, app.Type, "expected template names to match")

		})
	}
}

func TestInit_Prepare(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputOpts              InitAppOpts
		mocking                func()
		wantedExistingProjects []string
	}{
		"with existing projects": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
			wantedExistingProjects: []string{"project1", "project2"},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return([]*archer.Project{
						&archer.Project{Name: "project1"},
						&archer.Project{Name: "project2"},
					}, nil)

			},
		},
		"with error loading projects": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
			wantedExistingProjects: []string{},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error loading projects"))

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			tc.inputOpts.projStore = mockProjectStore
			tc.inputOpts.Prepare()
			require.ElementsMatch(t, tc.wantedExistingProjects, tc.inputOpts.existingProjects)
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

//TODO this test currently doesn't mock out the manifest writer.
// Since that part will change soon, I don't have tests for the
// manifest writer parts yet.
func TestInit_InitApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputOpts InitAppOpts
		mocking   func()
		wantedErr error
	}{
		"with an existing project": {
			inputOpts: InitAppOpts{
				Name:             "frontend",
				Project:          "project1",
				Type:             "Empty",
				existingProjects: []string{"project1", "project2"},
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(nil).
					Times(0)
			},
		},
		"with a new project": {
			inputOpts: InitAppOpts{
				Name:             "frontend",
				Project:          "project3",
				Type:             "Empty",
				existingProjects: []string{"project1", "project2"},
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
					Return(nil)
			},
		},
		"with an error creating a new project": {
			inputOpts: InitAppOpts{
				Name:             "frontend",
				Project:          "project3",
				Type:             "Empty",
				existingProjects: []string{"project1", "project2"},
			},
			wantedErr: fmt.Errorf("error creating project"),
			mocking: func() {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
					Return(fmt.Errorf("error creating project"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			tc.inputOpts.projStore = mockProjectStore
			err := tc.inputOpts.InitApp()
			if tc.wantedErr == nil {
				require.NoError(t, err, "There should be no error")
			} else {
				require.Error(t, tc.wantedErr, err.Error())
			}
		})
	}
}

func TestInit_DeployEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputProject string
		input        func(c *expect.Console)
		mocking      func()
		wantedErr    error
	}{
		"when there are no envs for a project": {
			// When a project is first created, and there are
			// no environments in it - we can offer to create
			// an env for the user.
			inputProject: "project",
			input: func(c *expect.Console) {
				c.ExpectString("Would you like to set up a test environment?")
				c.SendLine("n")
				c.ExpectEOF()
			},
			mocking: func() {
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("project")).
					Return([]*archer.Environment{}, nil)
			},
		},
		"when there are existing envs for a project": {
			// When a project already has environments, we don't
			// prompt the user to create a "test" env
			inputProject: "project",
			input: func(c *expect.Console) {
				c.ExpectEOF()
			},
			mocking: func() {
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("project")).
					Return([]*archer.Environment{
						&archer.Environment{Name: "test"},
					}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			app := &InitAppOpts{
				Project:  tc.inputProject,
				envStore: mockEnvStore,
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

			tc.mocking()

			// WHEN
			err := app.DeployEnv()

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, tc.wantedErr, err)
			}

		})
	}
}
