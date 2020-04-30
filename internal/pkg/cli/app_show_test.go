// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showAppMocks struct {
	storeSvc  *climocks.MockstoreReader
	prompt    *climocks.Mockprompter
	describer *climocks.Mockdescriber
	ws        *climocks.MockwsAppReader
}

type mockDescribeData struct {
	data string
	err  error
}

func (m *mockDescribeData) HumanString() string {
	return m.data
}

func (m *mockDescribeData) JSONString() (string, error) {
	return m.data, m.err
}

func TestAppShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		setupMocks       func(mocks showAppMocks)

		wantedError error
	}{
		"valid project name and application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-project").Return(&archer.Project{
						Name: "my-project",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-project", "my-app").Return(&archer.Application{
						Name: "my-app",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"invalid project name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-project").Return(&archer.Project{
						Name: "my-project",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-project", "my-app").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)

			mocks := showAppMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			showApps := &showAppOpts{
				showAppVars: showAppVars{
					appName: tc.inputApplication,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				storeSvc: mockStoreReader,
			}

			// WHEN
			err := showApps.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppShow_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject string
		inputApp     string

		setupMocks func(mocks showAppMocks)

		wantedProject string
		wantedApp     string
		wantedError   error
	}{
		"with all flags": {
			inputProject: "my-project",
			inputApp:     "my-app",
			setupMocks:   func(mocks showAppMocks) {},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"retrieve all app names if fail to retrieve app name from local": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askAppName
					m.ws.EXPECT().AppNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-project").Return([]*archer.Application{
						{Name: "my-app"},
						{Name: "archer-app"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("my-app", nil).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"retrieve all app names if no app found in local dir": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askAppName
					m.ws.EXPECT().AppNames().Return([]string{}, nil),
					m.storeSvc.EXPECT().ListServices("my-project").Return([]*archer.Application{
						{Name: "my-app"},
						{Name: "archer-app"},
					}, nil),

					m.prompt.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("my-app", nil).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"retrieve local app names": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askAppName
					m.ws.EXPECT().AppNames().Return([]string{"my-app", "archer-app"}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("my-app", nil).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"skip selecting if only one application found": {
			inputProject: "my-project",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.ws.EXPECT().AppNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-project").Return([]*archer.Application{
						{
							Name: "my-app",
						},
					}, nil),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"returns error when fail to list project": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().ListApplications().Return(nil, errors.New("some error"))
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("list projects: some error"),
		},
		"returns error when no project found": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{}, nil)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("no project found: run `project init` please"),
		},
		"returns error when fail to select project": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("", errors.New("some error")).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("select projects: some error"),
		},
		"returns error when fail to list applications": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askAskName
					m.ws.EXPECT().AppNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-project").Return(nil, fmt.Errorf("some error")),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("list applications for project my-project: some error"),
		},
		"returns error when fail to select application": {
			inputProject: "",
			inputApp:     "",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListApplications().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askAppName
					m.ws.EXPECT().AppNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-project").Return([]*archer.Application{
						{Name: "my-app"},
						{Name: "archer-app"},
					}, nil),

					m.prompt.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("", fmt.Errorf("some error")).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("select applications for project my-project: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)
			mockWorkspace := climocks.NewMockwsAppReader(ctrl)

			mocks := showAppMocks{
				storeSvc: mockStoreReader,
				prompt:   mockPrompter,
				ws:       mockWorkspace,
			}

			tc.setupMocks(mocks)

			showApps := &showAppOpts{
				showAppVars: showAppVars{
					appName: tc.inputApp,
					GlobalOpts: &GlobalOpts{
						prompt:      mockPrompter,
						projectName: tc.inputProject,
					},
				},
				storeSvc: mockStoreReader,
				ws:       mockWorkspace,
			}

			// WHEN
			err := showApps.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedProject, showApps.ProjectName(), "expected project name to match")
				require.Equal(t, tc.wantedApp, showApps.appName, "expected application name to match")
			}
		})
	}
}

func TestAppShow_Execute(t *testing.T) {
	projectName := "my-project"
	webApp := mockDescribeData{
		data: "mockData",
		err:  errors.New("some error"),
	}
	testCases := map[string]struct {
		inputApp         string
		shouldOutputJSON bool

		setupMocks func(mocks showAppMocks)

		wantedContent string
		wantedError   error
	}{
		"noop if app name is empty": {
			setupMocks: func(m showAppMocks) {
				m.describer.EXPECT().Describe().Times(0)
			},
		},
		"success": {
			inputApp: "my-app",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&webApp, nil),
				)
			},

			wantedContent: "mockData",
		},
		"return error if fail to generate JSON output": {
			inputApp:         "my-app",
			shouldOutputJSON: true,

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&webApp, nil),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to describe application": {
			inputApp: "my-app",

			setupMocks: func(m showAppMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("describe application my-app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockAppDescriber := climocks.NewMockdescriber(ctrl)

			mocks := showAppMocks{
				describer: mockAppDescriber,
			}

			tc.setupMocks(mocks)

			showApps := &showAppOpts{
				showAppVars: showAppVars{
					appName:          tc.inputApp,
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						projectName: projectName,
					},
				},
				describer:     mockAppDescriber,
				initDescriber: func(bool) error { return nil },
				w:             b,
			}

			// WHEN
			err := showApps.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
