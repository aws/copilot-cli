// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppShow_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject string
		inputApp     string

		mockStoreReader func(m *climocks.MockstoreReader)
		mockPrompt      func(m *climocks.Mockprompter)

		wantedProject string
		wantedApp     string
		wantedError   error
	}{
		"with all flags": {
			inputProject: "my-project",
			inputApp:     "my-app",

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"prompt for all input": {
			inputProject: "",
			inputApp:     "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "archer-app",
					},
				}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
				m.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("my-app", nil).Times(1)
			},
			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   nil,
		},
		"returns error when fail to list project": {
			inputProject: "",
			inputApp:     "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return(nil, errors.New("some error"))
			},

			mockPrompt:    func(m *climocks.Mockprompter) {},
			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("listing projects: some error"),
		},
		"returns error when fail to select project": {
			inputProject: "",
			inputApp:     "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("", errors.New("some error")).Times(1)
			},
			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("selecting projects: some error"),
		},
		"returns error when fail to list applications": {
			inputProject: "",
			inputApp:     "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
				m.EXPECT().ListApplications("my-project").Return(nil, fmt.Errorf("some error"))
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
			},
			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("listing applications for project my-project: some error"),
		},
		"returns error when fail to select application": {
			inputProject: "",
			inputApp:     "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "archer-app",
					},
				}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
				m.EXPECT().SelectOne(fmt.Sprintf(applicationShowAppNamePrompt, "my-project"), applicationShowAppNameHelpPrompt, []string{"my-app", "archer-app"}).Return("", fmt.Errorf("some error")).Times(1)
			},
			wantedProject: "my-project",
			wantedApp:     "my-app",
			wantedError:   fmt.Errorf("selecting applications for project my-project: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)
			tc.mockPrompt(mockPrompter)
			tc.mockStoreReader(mockStoreReader)

			showApps := &ShowAppOpts{
				appName:  tc.inputApp,
				storeSvc: mockStoreReader,
				GlobalOpts: &GlobalOpts{
					prompt:      mockPrompter,
					projectName: tc.inputProject,
				},
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
func TestAppShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		mockStoreReader  func(m *climocks.MockstoreReader)

		wantedError error
	}{
		"valid project name and application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{&archer.Project{
					Name: "my-project",
				}}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{&archer.Application{
					Name: "my-app",
				}}, nil)
			},

			wantedError: nil,
		},
		"invalid project name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{&archer.Project{
					Name: "my-bad-project",
				}}, nil)
			},

			wantedError: fmt.Errorf("project 'my-project' does not exist in the workspace"),
		},
		"invalid application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{&archer.Project{
					Name: "my-project",
				}}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{&archer.Application{
					Name: "my-bad-app",
				}}, nil)
			},

			wantedError: fmt.Errorf("application 'my-app' does not exist in project 'my-project'"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			tc.mockStoreReader(mockStoreReader)

			showApps := &ShowAppOpts{
				storeSvc: mockStoreReader,

				appName: tc.inputApplication,
				GlobalOpts: &GlobalOpts{
					projectName: tc.inputProject,
				},
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

func TestAppShow_Execute(t *testing.T) {
	projectName := "my-project"
	testCases := map[string]struct {
		inputApp         string
		shouldOutputJSON bool

		mockStoreReader     func(m *climocks.MockstoreReader)
		mockWebAppDescriber func(m *climocks.MockwebAppDescriber)

		wantedContent string
		wantedError   error
	}{
		"prompt for all input for json output": {
			inputApp:         "my-app",
			shouldOutputJSON: true,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name: "test",
					},
					&archer.Environment{
						Name: "prod",
					},
				}, nil)
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {
				m.EXPECT().URI("test").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/frontend",
				}, nil)
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
				m.EXPECT().ECSParams("test").Return(&describe.WebAppECSParams{
					ContainerPort: "80",
					TaskSize: describe.TaskSize{
						CPU:    "256",
						Memory: "512",
					},
					TaskCount: "1",
				}, nil)
				m.EXPECT().ECSParams("prod").Return(&describe.WebAppECSParams{
					ContainerPort: "5000",
					TaskSize: describe.TaskSize{
						CPU:    "512",
						Memory: "1024",
					},
					TaskCount: "3",
				}, nil)
			},

			wantedContent: "{\"appName\":\"my-app\",\"type\":\"\",\"project\":\"my-project\",\"deployConfig\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\",\"url\":\"my-pr-Publi.us-west-2.elb.amazonaws.com\",\"path\":\"/frontend\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\",\"url\":\"my-pr-Publi.us-west-2.elb.amazonaws.com\",\"path\":\"/backend\"}]}\n",
		},
		"prompt for all input for human output": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name: "test",
					},
					&archer.Environment{
						Name: "prod",
					},
				}, nil)
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {
				m.EXPECT().URI("test").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/frontend",
				}, nil)
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
				m.EXPECT().ECSParams("test").Return(&describe.WebAppECSParams{
					ContainerPort: "80",
					TaskSize: describe.TaskSize{
						CPU:    "256",
						Memory: "512",
					},
					TaskCount: "1",
				}, nil)
				m.EXPECT().ECSParams("prod").Return(&describe.WebAppECSParams{
					ContainerPort: "5000",
					TaskSize: describe.TaskSize{
						CPU:    "512",
						Memory: "1024",
					},
					TaskCount: "3",
				}, nil)
			},

			wantedContent: `About

  Project           my-project
  Name              my-app
  Type              

Configurations

  Environment       CPU (vCPU)          Memory (MiB)        Port                Tasks
  test              0.25                512                 80                  1
  prod              0.5                 1024                5000                3

Routes

  Environment       URL                                      Path
  test              my-pr-Publi.us-west-2.elb.amazonaws.com  /frontend
  prod              my-pr-Publi.us-west-2.elb.amazonaws.com  /backend
`,
		},
		"returns error if fail to get application": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(nil, errors.New("some error"))
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {},

			wantedError: fmt.Errorf("getting application: some error"),
		},
		"returns error if fail to list environments": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return(nil, errors.New("some error"))
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {},

			wantedError: fmt.Errorf("listing environments: some error"),
		},
		"do not return error if no application found with json format": {
			inputApp:         "",
			shouldOutputJSON: true,

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {},

			wantedContent: "{\"appName\":\"\",\"type\":\"\",\"project\":\"\",\"deployConfig\":null}\n",
		},
		"do not return error if no application found": {
			inputApp:         "",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {},

			wantedContent: "About\n\n  Project           \n  Name              \n  Type              \n\nConfigurations\n\n  Environment       CPU (vCPU)          Memory (MiB)        Port                Tasks\n\nRoutes\n\n  Environment       URL                 Path\n",
		},
		"returns error if fail to retrieve URI": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name: "test",
					},
					&archer.Environment{
						Name: "prod",
					},
				}, nil)
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {
				m.EXPECT().URI("test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("retrieving application URI: some error"),
		},
		"returns error if fail to retrieve deploy info": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name: "test",
					},
					&archer.Environment{
						Name: "prod",
					},
				}, nil)
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {
				m.EXPECT().URI("test").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/frontend",
				}, nil)
				m.EXPECT().ECSParams("test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("retrieving application deployment configuration: some error"),
		},
		"do not return error if fail to retrieve URI because of application not deployed": {
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name: "test",
					},
					&archer.Environment{
						Name: "prod",
					},
				}, nil)
			},

			mockWebAppDescriber: func(m *climocks.MockwebAppDescriber) {
				m.EXPECT().URI("test").Return(nil, fmt.Errorf("describe stack my-project-test-my-app: %w", awserr.New("ValidationError", "Stack with id my-project-test-my-app does not exist", nil)))
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
				m.EXPECT().ECSParams("test").Times(0)
				m.EXPECT().ECSParams("prod").Return(&describe.WebAppECSParams{
					ContainerPort: "5000",
					TaskSize: describe.TaskSize{
						CPU:    "512",
						Memory: "1024",
					},
					TaskCount: "3",
				}, nil)
			},

			wantedContent: "About\n\n  Project           my-project\n  Name              my-app\n  Type              \n\nConfigurations\n\n  Environment       CPU (vCPU)          Memory (MiB)        Port                Tasks\n  prod              0.5                 1024                5000                3\n\nRoutes\n\n  Environment       URL                                      Path\n  prod              my-pr-Publi.us-west-2.elb.amazonaws.com  /backend\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockWebAppDescriber := climocks.NewMockwebAppDescriber(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockWebAppDescriber(mockWebAppDescriber)

			showApps := &ShowAppOpts{
				appName:          tc.inputApp,
				ShouldOutputJSON: tc.shouldOutputJSON,

				storeSvc:  mockStoreReader,
				describer: mockWebAppDescriber,

				w: b,

				GlobalOpts: &GlobalOpts{
					projectName: projectName,
				},
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
