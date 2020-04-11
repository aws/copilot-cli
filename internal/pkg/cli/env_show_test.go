// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type showEnvMocks struct {
	storeSvc *climocks.MockstoreReader
	prompt   *climocks.Mockprompter
	//describer   *climocks.MockwebEnvDescriber
}

func TestEnvShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject     string
		inputEnvironment string
		setupMocks       func(mocks showEnvMocks)

		wantedError error
	}{
		"valid project name and environment name": {
			inputProject:     "my-project",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetProject("my-project").Return(&archer.Project{
						Name: "my-project",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-project", "my-env").Return(&archer.Environment{
						Name: "my-env",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"invalid project name": {
			inputProject:     "my-project",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().GetProject("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputProject:     "my-project",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetProject("my-project").Return(&archer.Project{
						Name: "my-project",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-project", "my-env").Return(nil, errors.New("some error")),
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

			mocks := showEnvMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				storeSvc: mockStoreReader,
			}

			// WHEN
			err := showEnvs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestEnvShow_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject string
		inputEnv     string

		setupMocks func(mocks showEnvMocks) //or m?

		wantedProject string
		wantedEnv     string
		wantedError   error
	}{
		"with all flags": {
			inputProject: "my-project",
			inputEnv:     "my-env",

			setupMocks: func(mocks showEnvMocks) {},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   nil,
		},
		"retrieve all env names": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askEnvName
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
						{Name: "my-env"},
						{Name: "archer-env"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(environmentShowEnvNamePrompt, "my-project"), environmentShowEnvNameHelpPrompt, []string{"my-env", "archer-env"}).Return("my-env", nil).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   nil,
		},
		"skip selecting if only one project found": {
			inputProject: "",
			inputEnv:     "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{
						{
							Name: "my-project",
						},
					}, nil),
				)
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   nil,
		},
		"skip selecting if only one env found": {
			inputProject: "my-project",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
						{
							Name: "my-env",
						},
					}, nil),
				)
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   nil,
		},
		"returns error when fail to list project": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().ListProjects().Return(nil, errors.New("some error"))
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("list projects: some error"),
		},
		"returns error when no project found": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{}, nil)
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("no project found: run `project init` please"),
		},
		"returns error when fail to select project": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("", errors.New("some error")).Times(1),
				)
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("select projects: some error"),
		},
		"returns error when fail to list environments": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),
					//askEnvName
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return(nil, fmt.Errorf("some error")),
				)
			},
			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("list environments for project my-project: some error"),
		},
		"returns error when fail to select environment": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					// askProject
					m.storeSvc.EXPECT().ListProjects().Return([]*archer.Project{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),
					//askEnvName
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*archer.Environment{
						{Name: "my-env"},
						{Name: "archer-env"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(environmentShowEnvNamePrompt, "my-project"), environmentShowEnvNameHelpPrompt, []string{"my-env", "archer-env"}).Return("", fmt.Errorf("some error")).Times(1),
				)
			},
			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("select environment for project my-project: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)

			mocks := showEnvMocks{
				storeSvc: mockStoreReader,
				prompt:   mockPrompter,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					envName: tc.inputEnv,
					GlobalOpts: &GlobalOpts{
						prompt:      mockPrompter,
						projectName: tc.inputProject,
					},
				},
				storeSvc: mockStoreReader,
			}
			// WHEN
			err := showEnvs.Ask()
			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedProject, showEnvs.ProjectName(), "expected project name to match")
				require.Equal(t, tc.wantedEnv, showEnvs.envName, "expected environment name to match")
			}
		})
	}
}
