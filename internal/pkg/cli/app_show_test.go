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
		inputProject    string
		mockStoreReader func(m *climocks.MockstoreReader)

		wantedError error
	}{
		"valid project name": {
			inputProject: "my-project",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name: "my-project",
				}, nil)
			},

			wantedError: nil,
		},
		"invalid project name": {
			inputProject: "my-project",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("getting project: some error"),
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
	testCases := map[string]struct {
		inputProject     *archer.Project
		inputApp         string
		shouldOutputJSON bool

		mockStoreReader        func(m *climocks.MockstoreReader)
		mockResourceIdentifier func(m *climocks.MockresourceIdentifier)

		wantedContent string
		wantedError   error
	}{
		"prompt for all input for json output": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
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

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {
				m.EXPECT().URI("test").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/frontend",
				}, nil)
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
			},

			wantedContent: "{\"appName\":\"my-app\",\"type\":\"\",\"project\":\"my-project\",\"account\":\"\",\"environments\":[{\"name\":\"test\",\"region\":\"\",\"prod\":false,\"url\":\"my-pr-Publi.us-west-2.elb.amazonaws.com\",\"path\":\"/frontend\"},{\"name\":\"prod\",\"region\":\"\",\"prod\":false,\"url\":\"my-pr-Publi.us-west-2.elb.amazonaws.com\",\"path\":\"/backend\"}]}\n",
		},
		"prompt for all input for human output": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
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

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {
				m.EXPECT().URI("test").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/frontend",
				}, nil)
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
			},

			wantedContent: `Environment         Is Production?      Path                URL
-----------         --------------      ---------           ---------------------------------------
test                false               /frontend           my-pr-Publi.us-west-2.elb.amazonaws.com
prod                false               /backend            my-pr-Publi.us-west-2.elb.amazonaws.com
`,
		},
		"returns error if fail to get application": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(nil, errors.New("some error"))
			},

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {},

			wantedError: fmt.Errorf("getting application: some error"),
		},
		"returns error if fail to list environments": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
			inputApp:         "my-app",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return(nil, errors.New("some error"))
			},

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {},

			wantedError: fmt.Errorf("listing environments: some error"),
		},
		"do not return error if no application found with json format": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
			inputApp:         "",
			shouldOutputJSON: true,

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {},

			wantedContent: "{\"appName\":\"\",\"type\":\"\",\"project\":\"\",\"account\":\"\",\"environments\":null}\n",
		},
		"do not return error if no application found": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
			inputApp:         "",
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {},

			wantedContent: `Environment         Is Production?      Path                URL
-----------         --------------      ----                ---
`,
		},
		"returns error if fail to retrieve URI": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
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

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {
				m.EXPECT().URI("test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("retrieving application URI: some error"),
		},
		"do not return error if fail to retrieve URI because of application not deployed": {
			inputProject: &archer.Project{
				Name: "my-project",
			},
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

			mockResourceIdentifier: func(m *climocks.MockresourceIdentifier) {
				m.EXPECT().URI("test").Return(nil, fmt.Errorf("describe stack my-project-test-my-app: %w", awserr.New("ValidationError", "Stack with id my-project-test-my-app does not exist", nil)))
				m.EXPECT().URI("prod").Return(&describe.WebAppURI{
					DNSName: "my-pr-Publi.us-west-2.elb.amazonaws.com",
					Path:    "/backend",
				}, nil)
			},

			wantedContent: `Environment         Is Production?      Path                URL
-----------         --------------      --------            ---------------------------------------
prod                false               /backend            my-pr-Publi.us-west-2.elb.amazonaws.com
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockResourceIdentifier := climocks.NewMockresourceIdentifier(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockResourceIdentifier(mockResourceIdentifier)

			showApps := &ShowAppOpts{
				proj:             tc.inputProject,
				appName:          tc.inputApp,
				ShouldOutputJSON: tc.shouldOutputJSON,

				storeSvc:   mockStoreReader,
				identifier: mockResourceIdentifier,

				w: b,

				GlobalOpts: &GlobalOpts{
					projectName: tc.inputProject.Name,
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
