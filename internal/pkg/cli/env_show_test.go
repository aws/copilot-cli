// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"

	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showEnvMocks struct {
	storeSvc  *mocks.Mockstore
	prompt    *mocks.Mockprompter
	describer *mocks.MockenvDescriber
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
					m.storeSvc.EXPECT().GetApplication("my-project").Return(&config.Application{
						Name: "my-project",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-project", "my-env").Return(&config.Environment{
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
				m.storeSvc.EXPECT().GetApplication("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputProject:     "my-project",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-project").Return(&config.Application{
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

			mockStoreReader := mocks.NewMockstore(ctrl)

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
				store: mockStoreReader,
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

		setupMocks func(mocks showEnvMocks)

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
					m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),

					// askEnvName
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*config.Environment{
						{Name: "my-env"},
						{Name: "archer-env"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtEnvironmentShowEnvNamePrompt, "my-project"), environmentShowEnvNameHelpPrompt, []string{"my-env", "archer-env"}).Return("my-env", nil).Times(1),
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
					m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{
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
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*config.Environment{
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
				m.storeSvc.EXPECT().ListApplications().Return(nil, errors.New("some error"))
			},

			wantedProject: "my-project",
			wantedEnv:     "my-env",
			wantedError:   fmt.Errorf("list projects: some error"),
		},
		"returns error when no project found": {
			inputProject: "",
			inputEnv:     "",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{}, nil)
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
					m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{
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
					m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{
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
					m.storeSvc.EXPECT().ListApplications().Return([]*config.Application{
						{Name: "my-project"},
						{Name: "archer-project"},
					}, nil),
					m.prompt.EXPECT().SelectOne(environmentShowProjectNamePrompt, environmentShowProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1),
					//askEnvName
					m.storeSvc.EXPECT().ListEnvironments("my-project").Return([]*config.Environment{
						{Name: "my-env"},
						{Name: "archer-env"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtEnvironmentShowEnvNamePrompt, "my-project"), environmentShowEnvNameHelpPrompt, []string{"my-env", "archer-env"}).Return("", fmt.Errorf("some error")).Times(1),
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

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)

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
				store: mockStoreReader,
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

func TestEnvShow_Execute(t *testing.T) {

	mockApplications := []*config.Service{
		{App: "my-project",
			Name: "my-app",
			Type: "lb-web-app"},
		{App: "my-project",
			Name: "copilot-app",
			Type: "lb-web-app"},
	}
	mockTags := map[string]string{"tag1": "value1", "tag2": "value2"}

	mockEnv := &describe.EnvDescription{
		Environment: &config.Environment{
			App:              "my-project",
			Name:             "test",
			Region:           "us-west-2",
			AccountID:        "123456789",
			Prod:             false,
			RegistryURL:      "",
			ExecutionRoleARN: "",
			ManagerRoleARN:   "",
		},
		Services: mockApplications,
		Tags:     mockTags}

	testCases := map[string]struct {
		inputEnv         string
		shouldOutputJSON bool

		mockEnvDescriber func(m *mocks.MockenvDescriber)

		wantedContent string
		wantedError   error
	}{
		"correctly shows json output": {
			inputEnv:         "test",
			shouldOutputJSON: true,

			mockEnvDescriber: func(m *mocks.MockenvDescriber) {
				gomock.InOrder(
					m.EXPECT().Describe().Return(mockEnv, nil))
			},

			wantedContent: "{\"environment\":{\"app\":\"my-project\",\"name\":\"test\",\"region\":\"us-west-2\",\"accountID\":\"123456789\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"services\":[{\"App\":\"my-project\",\"name\":\"my-app\",\"type\":\"lb-web-app\"},{\"App\":\"my-project\",\"name\":\"copilot-app\",\"type\":\"lb-web-app\"}],\"tags\":{\"tag1\":\"value1\",\"tag2\":\"value2\"}}\n",
		},
		"correctly shows human output": {
			inputEnv:         "test",
			shouldOutputJSON: false,

			mockEnvDescriber: func(m *mocks.MockenvDescriber) {
				gomock.InOrder(
					m.EXPECT().Describe().Return(mockEnv, nil))
			},

			wantedContent: `About

  Name              test
  Production        false
  Region            us-west-2
  Account ID        123456789

Services

  Name              Type
  my-app            lb-web-app
  copilot-app       lb-web-app
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := mocks.NewMockstore(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			tc.mockEnvDescriber(mockEnvDescriber)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts:       &GlobalOpts{},
				},
				store:            mockStoreReader,
				describer:        mockEnvDescriber,
				initEnvDescriber: func(opts *showEnvOpts) error { return nil },
				w:                b,
			}

			// WHEN
			err := showEnvs.Execute()

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
