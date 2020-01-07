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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProjectShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject    string
		mockStoreReader func(m *climocks.MockstoreReader)

		wantedError error
	}{
		"valid project name and application name": {
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

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			tc.mockStoreReader(mockStoreReader)

			showProjects := &ShowProjectOpts{
				storeSvc: mockStoreReader,

				GlobalOpts: &GlobalOpts{
					projectName: tc.inputProject,
				},
			}

			// WHEN
			err := showProjects.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestProjectShow_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject string

		mockStoreReader func(m *climocks.MockstoreReader)
		mockPrompt      func(m *climocks.Mockprompter)

		wantedProject string
		wantedError   error
	}{
		"with all flags": {
			inputProject: "my-project",

			mockStoreReader: func(m *climocks.MockstoreReader) {},

			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedProject: "my-project",
			wantedError:   nil,
		},
		"prompt for all input": {
			inputProject: "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
			},
			wantedProject: "my-project",
			wantedError:   nil,
		},
		"returns error if fail to list project": {
			inputProject: "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return(nil, errors.New("some error"))
			},

			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("list project: some error"),
		},
		"returns error if fail to select project": {
			inputProject: "",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationShowProjectNamePrompt, applicationShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("", errors.New("some error")).Times(1)
			},

			wantedError: fmt.Errorf("select project: some error"),
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

			showProjects := &ShowProjectOpts{
				storeSvc: mockStoreReader,
				GlobalOpts: &GlobalOpts{
					prompt:      mockPrompter,
					projectName: tc.inputProject,
				},
			}

			// WHEN
			err := showProjects.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedProject, showProjects.ProjectName(), "expected project name to match")

			}
		})
	}
}

func TestProjectShow_Execute(t *testing.T) {
	projectName := "my-project"
	testCases := map[string]struct {
		shouldOutputJSON bool

		mockStoreReader func(m *climocks.MockstoreReader)

		wantedContent string
		wantedError   error
	}{
		"prompt for all input for json output": {
			shouldOutputJSON: true,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name:   "my-project",
					Domain: "example.com",
				}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
						Type: "lb-web-app",
					},
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					&archer.Environment{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
			},

			wantedContent: "{\"name\":\"my-project\",\"uri\":\"example.com\",\"environments\":[{\"name\":\"test\",\"accountID\":\"123456789\",\"region\":\"us-west-2\"},{\"name\":\"prod\",\"accountID\":\"123456789\",\"region\":\"us-west-1\"}],\"applications\":[{\"name\":\"my-app\",\"type\":\"lb-web-app\"}]}\n",
		},
		"prompt for all input for human output": {
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name:   "my-project",
					Domain: "example.com",
				}, nil)
				m.EXPECT().ListApplications("my-project").Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
						Type: "lb-web-app",
					},
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					&archer.Environment{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
			},

			wantedContent: `About

  Name              my-project
  URI               example.com

Environments

  Name              AccountID           Region
  test              123456789           us-west-2
  prod              123456789           us-west-1

Applications

  Name              Type
  my-app            lb-web-app
`,
		},
		"returns error if fail to get project": {
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("get project: some error"),
		},
		"returns error if fail to list environment": {
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name:   "my-project",
					Domain: "example.com",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("list environment: some error"),
		},
		"returns error if fail to list application": {
			shouldOutputJSON: false,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name:   "my-project",
					Domain: "example.com",
				}, nil)
				m.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
					&archer.Environment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					&archer.Environment{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.EXPECT().ListApplications("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("list application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			tc.mockStoreReader(mockStoreReader)

			showProjects := &ShowProjectOpts{
				shouldOutputJSON: tc.shouldOutputJSON,

				storeSvc: mockStoreReader,

				w: b,

				GlobalOpts: &GlobalOpts{
					projectName: projectName,
				},
			}

			// WHEN
			err := showProjects.Execute()

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
