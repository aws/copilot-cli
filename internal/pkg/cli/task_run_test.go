// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type basicOpts struct {
	inCount  int
	inCPU    int
	inMemory int
}

var defaultOpts = basicOpts{
	inCount:  1,
	inCPU:    256,
	inMemory: 512,
}

// NOTE: mock spinner so that it doesn't create log output when testing Execute
type mockSpinner struct{}

func (s *mockSpinner) Start(label string)           {}
func (s *mockSpinner) Stop(label string)            {}
func (s *mockSpinner) Events([]termprogress.TabRow) {}

type runTaskMocks struct {
	deployer   *mocks.MocktaskDeployer
	repository *mocks.MockrepositoryService
	runner     *mocks.MocktaskRunner
}

func TestTaskRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		basicOpts

		inName string

		inImage          string
		inDockerfilePath string

		inTaskRole string

		inEnv            string
		inSubnets        []string
		inSecurityGroups []string

		inEnvVars map[string]string
		inCommand string

		appName string

		mockStore      func(m *mocks.Mockstore)
		mockFileSystem func(mockFS afero.Fs)

		wantedError error
	}{
		"valid with no flag": {
			basicOpts:   defaultOpts,
			wantedError: nil,
		},
		"valid with flags image and env": {
			basicOpts: defaultOpts,

			inName: "my-task",

			inImage:    "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			inTaskRole: "exec-role",

			inEnv: "dev",

			inEnvVars: map[string]string{
				"NAME": "my-app",
				"ENV":  "dev",
			},
			inCommand: "echo hello world",

			appName: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)

				m.EXPECT().GetEnvironment("my-app", "dev").Return(&config.Environment{
					App:  "my-app",
					Name: "dev",
				}, nil)
			},

			wantedError: nil,
		},
		"valid without flags image and env": {
			basicOpts: defaultOpts,

			inName: "my-task",

			inDockerfilePath: "hello/world/Dockerfile",
			inTaskRole:       "exec-role",

			inSubnets:        []string{"subnet-10d938jds"},
			inSecurityGroups: []string{"sg-0d9sjdk", "sg-d33kds99"},

			inEnvVars: map[string]string{
				"NAME": "pj",
				"ENV":  "dev",
			},
			inCommand: "echo hello world",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello/world", 0755)
				afero.WriteFile(mockFS, "hello/world/Dockerfile", []byte("FROM nginx"), 0644)
			},
			wantedError: nil,
		},
		"invalid number of tasks": {
			basicOpts: basicOpts{
				inCount:  -1,
				inCPU:    256,
				inMemory: 512,
			},
			wantedError: errNumNotPositive,
		},
		"invalid number of CPU units": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    -15,
				inMemory: 512,
			},
			wantedError: errCpuNotPositive,
		},
		"invalid memory": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    256,
				inMemory: -1024,
			},
			wantedError: errMemNotPositive,
		},
		"both dockerfile and image name specified": {
			basicOpts: defaultOpts,

			inImage:          "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			inDockerfilePath: "hello/world/Dockerfile",

			wantedError: errors.New("cannot specify both image and Dockerfile path"),
		},
		"invalid dockerfile path": {
			basicOpts: defaultOpts,

			inDockerfilePath: "world/hello/Dockerfile",
			wantedError:      errors.New("open world/hello/Dockerfile: file does not exist"),
		},
		"specified app exists": {
			basicOpts: defaultOpts,

			appName: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},

			wantedError: nil,
		},
		"unknown app": {
			basicOpts: defaultOpts,

			appName: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "my-app",
					AccountID:       "115",
					Region:          "us-east-1",
				})
			},
			wantedError: errors.New("get application: couldn't find an application named my-app in account 115 and region us-east-1"),
		},
		"env exists in app": {
			basicOpts: defaultOpts,

			appName: "my-app",
			inEnv:   "dev",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "dev").Return(&config.Environment{
					App:  "my-app",
					Name: "dev",
				}, nil)

				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: nil,
		},
		"unknown env in app": {
			basicOpts: defaultOpts,

			appName: "my-app",
			inEnv:   "dev",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "dev").Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: "my-app",
					EnvironmentName: "dev",
				})

				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: errors.New("get environment dev config: couldn't find environment dev in the application my-app"),
		},
		"no workspace": {
			basicOpts: defaultOpts,

			inEnv:       "test",
			wantedError: errNoAppInWorkspace,
		},
		"both environment and subnet id specified": {
			basicOpts: defaultOpts,

			inEnv:     "test",
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("neither subnet nor security groups should be specified if environment is specified"),
		},
		"both environment and security groups specified": {
			basicOpts: defaultOpts,

			inEnv:            "test",
			inSecurityGroups: []string{"security group id1", "securty group id2"},

			wantedError: errors.New("neither subnet nor security groups should be specified if environment is specified"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.appName,
					},
					count:          tc.inCount,
					cpu:            tc.inCPU,
					memory:         tc.inMemory,
					groupName:      tc.inName,
					image:          tc.inImage,
					env:            tc.inEnv,
					taskRole:       tc.inTaskRole,
					subnets:        tc.inSubnets,
					securityGroups: tc.inSecurityGroups,
					dockerfilePath: tc.inDockerfilePath,
					envVars:        tc.inEnvVars,
					command:        tc.inCommand,
				},
				fs:    &afero.Afero{Fs: afero.NewMemMapFs()},
				store: mockStore,
			}

			if tc.mockFileSystem != nil {
				tc.mockFileSystem(opts.fs)
			}
			if tc.mockStore != nil {
				tc.mockStore(mockStore)
			}

			err := opts.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskRunOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inName string

		inEnv   string
		appName string

		mockSel    func(m *mocks.MockappEnvSelector)
		mockPrompt func(m *mocks.Mockprompter)

		wantedError error
		wantedApp   string
		wantedEnv   string
		wantedName  string
	}{
		"selected an existing application": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), appEnvOptionNone).Return("app", nil)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantedApp: "app",
		},
		"selected None app": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), appEnvOptionNone).Return(appEnvOptionNone, nil)
			},
			wantedApp: "",
		},
		"don't prompt for app when under a workspace or app flag is specified": {
			appName: "my-app",
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).Times(1)
			},
			wantedApp: "my-app",
		},
		"selected an existing environment": {
			appName: "my-app",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(),
					"my-app", appEnvOptionNone).Return("test", nil)
			},

			wantedEnv: "test",
			wantedApp: "my-app",
		},
		"selected None env": {
			appName: "my-app",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(),
					"my-app", appEnvOptionNone).Return(appEnvOptionNone, nil)
			},

			wantedEnv: "",
			wantedApp: "my-app",
		},
		"don't prompt if env is provided": {
			inEnv:   "test",
			appName: "my-app",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: "test",
			wantedApp: "my-app",
		},
		"don't prompt if no workspace or selected None app": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any(), appEnvOptionNone).Return(appEnvOptionNone, nil)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: "",
		},
		"prompt for task family name": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(taskRunGroupNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("my-task", nil)
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},

			wantedName: "my-task",
		},
		"error getting task group name": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("error getting task group name"))
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("prompt get task group name: error getting task group name"),
		},
		"error selecting environment": {
			appName: "my-app",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).
					Return("", fmt.Errorf("error selecting environment"))
			},

			wantedError: errors.New("ask for environment: error selecting environment"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockappEnvSelector(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)

			if tc.mockSel != nil {
				tc.mockSel(mockSel)
			}

			if tc.mockPrompt != nil {
				tc.mockPrompt(mockPrompter)
			}

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.appName,
						prompt:  mockPrompter,
					},
					groupName: tc.inName,
					env:       tc.inEnv,
				},
				sel: mockSel,
			}

			err := opts.Ask()

			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedEnv, opts.env)
				require.Equal(t, tc.wantedApp, opts.AppName())
				if tc.wantedName != "" {
					require.Equal(t, tc.wantedName, opts.groupName)
				}
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}

func TestTaskRunOpts_Execute(t *testing.T) {
	const inGroupName = "my-task"
	mockRepoURI := "uri/repo"

	tag := "tag"

	defaultBuildArguments := docker.BuildArguments{
		Context:  filepath.Dir(defaultDockerfilePath),
		ImageTag: imageTagLatest,
	}

	testCases := map[string]struct {
		inImage string
		inTag   string

		setupMocks func(m runTaskMocks)

		wantedError error
	}{
		"error deploying resources": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(&deploy.CreateTaskResourcesInput{
					Name:  inGroupName,
					Image: "",
				}).Return(errors.New("error deploying"))
			},
			wantedError: errors.New("provision resources for task my-task: error deploying"),
		},
		"error updating resources": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(&deploy.CreateTaskResourcesInput{
					Name:  inGroupName,
					Image: "",
				}).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI)
				m.deployer.EXPECT().DeployTask(&deploy.CreateTaskResourcesInput{
					Name:  inGroupName,
					Image: "uri/repo:latest",
				}).Times(1).Return(errors.New("error updating"))
			},
			wantedError: errors.New("update resources for task my-task: error updating"),
		},
		"error running tasks": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(gomock.Any()).Return(nil).Times(2)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI)
				m.runner.EXPECT().Run().Return(nil, errors.New("error running"))
			},
			wantedError: errors.New("run task my-task: error running"),
		},
		"append 'latest' to image tag": {
			inTag: tag,
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(gomock.Any()).AnyTimes()
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(
					&docker.BuildArguments{
						Context:        filepath.Dir(defaultDockerfilePath),
						ImageTag:       imageTagLatest,
						AdditionalTags: []string{tag},
					}),
				)
				m.repository.EXPECT().URI().AnyTimes()
				m.runner.EXPECT().Run().AnyTimes()
			},
		},
		"update image to task resource if image is not provided": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(&deploy.CreateTaskResourcesInput{
					Name:  inGroupName,
					Image: "",
				}).Times(1).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI)
				m.deployer.EXPECT().DeployTask(&deploy.CreateTaskResourcesInput{
					Name:  inGroupName,
					Image: "uri/repo:latest",
				}).Times(1).Return(nil)
				m.runner.EXPECT().Run().AnyTimes()
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDeployer := mocks.NewMocktaskDeployer(ctrl)
			mockRepo := mocks.NewMockrepositoryService(ctrl)
			mockRunner := mocks.NewMocktaskRunner(ctrl)

			mocks := runTaskMocks{
				deployer:   mockDeployer,
				repository: mockRepo,
				runner:     mockRunner,
			}
			tc.setupMocks(mocks)

			opts := &runTaskOpts{
				runTaskVars: runTaskVars{
					groupName: inGroupName,
					image:     tc.inImage,
					imageTag:  tc.inTag,
				},
				spinner:  &mockSpinner{},
				deployer: mockDeployer,
			}
			opts.configureRuntimeOpts = func() error {
				opts.repository = mockRepo
				opts.runner = mockRunner
				return nil
			}

			err := opts.Execute()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
