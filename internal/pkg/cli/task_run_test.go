// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/task"

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

func (s *mockSpinner) Start(label string) {}
func (s *mockSpinner) Stop(label string)  {}

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

		inEnvVars    map[string]string
		inCommand    string
		inEntryPoint string

		inDefault bool

		appName         string
		isDockerfileSet bool

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
			inEntryPoint: "exec 'enter here'",

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
			wantedError: errCPUNotPositive,
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

			inImage:         "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			isDockerfileSet: true,

			wantedError: errors.New("cannot specify both `--image` and `--dockerfile`"),
		},
		"invalid dockerfile path": {
			basicOpts: defaultOpts,

			inDockerfilePath: "world/hello/Dockerfile",
			isDockerfileSet:  true,

			wantedError: errors.New("open world/hello/Dockerfile: file does not exist"),
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
		"both environment and subnets specified": {
			basicOpts: defaultOpts,

			inEnv:     "test",
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("cannot specify both `--subnets` and `--env`"),
		},
		"both environment and security groups specified": {
			basicOpts: defaultOpts,

			inEnv:            "test",
			inSecurityGroups: []string{"security group id1", "securty group id2"},

			wantedError: errors.New("cannot specify both `--security-groups` and `--env`"),
		},
		"both application and subnets specified": {
			basicOpts: defaultOpts,

			appName:   "my-app",
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("cannot specify both `--subnets` and `--app`"),
		},
		"both application and security groups specified": {
			basicOpts: defaultOpts,

			appName:          "my-app",
			inSecurityGroups: []string{"security group id1", "security group id2"},

			wantedError: errors.New("cannot specify both `--security-groups` and `--app`"),
		},
		"both default and subnets specified": {
			basicOpts: defaultOpts,

			inDefault: true,
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("cannot specify both `--subnets` and `--default`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					appName:           tc.appName,
					count:             tc.inCount,
					cpu:               tc.inCPU,
					memory:            tc.inMemory,
					groupName:         tc.inName,
					image:             tc.inImage,
					env:               tc.inEnv,
					taskRole:          tc.inTaskRole,
					subnets:           tc.inSubnets,
					securityGroups:    tc.inSecurityGroups,
					dockerfilePath:    tc.inDockerfilePath,
					envVars:           tc.inEnvVars,
					command:           tc.inCommand,
					entrypoint:        tc.inEntryPoint,
					useDefaultSubnets: tc.inDefault,
				},
				isDockerfileSet: tc.isDockerfileSet,

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
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskRunOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inName string

		inSubnets        []string
		inSecurityGroups []string

		inDefault bool
		inEnv     string
		appName   string

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
			appName:   "my-app",
			inDefault: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).AnyTimes()
			},
			wantedApp: "my-app",
		},
		"don't prompt for env if env is provided": {
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
		"don't prompt for env if no workspace and selected None app": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any(), appEnvOptionNone).Return(appEnvOptionNone, nil)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: "",
		},
		"don't prompt for app if using default": {
			inDefault: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).AnyTimes()
			},
			wantedApp: "",
		},
		"don't prompt for env if using default": {
			inDefault: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any(), appEnvOptionNone).AnyTimes()
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: "",
		},
		"don't prompt for app if subnets are specified": {
			inSubnets: []string{"subnet-1"},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).AnyTimes()
			},
		},
		"don't prompt for env if subnets are specified": {
			inSubnets: []string{"subnet-1"},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).Times(0)
			},
		},
		"don't prompt for app if security groups are specified": {
			inSecurityGroups: []string{"sg-1"},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).AnyTimes()
			},
		},
		"don't prompt for env if security groups are specified": {
			inSecurityGroups: []string{"sg-1"},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).Times(0)
			},
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
					appName:           tc.appName,
					groupName:         tc.inName,
					env:               tc.inEnv,
					useDefaultSubnets: tc.inDefault,
					subnets:           tc.inSubnets,
					securityGroups:    tc.inSecurityGroups,
				},
				sel: mockSel,
			}

			err := opts.Ask()

			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedEnv, opts.env)
				require.Equal(t, tc.wantedApp, opts.appName)
				if tc.wantedName != "" {
					require.Equal(t, tc.wantedName, opts.groupName)
				}
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}

type runTaskMocks struct {
	deployer             *mocks.MocktaskDeployer
	repository           *mocks.MockrepositoryService
	runner               *mocks.MocktaskRunner
	store                *mocks.Mockstore
	eventsWriter         *mocks.MockeventsWriter
	defaultClusterGetter *mocks.MockdefaultClusterGetter
	publicIPGetter       *mocks.MockpublicIPGetter
}

func mockHasDefaultCluster(m runTaskMocks) {
	m.defaultClusterGetter.EXPECT().HasDefaultCluster().Return(true, nil).AnyTimes()
}

func mockRepositoryAnytime(m runTaskMocks) {
	m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Any()).AnyTimes()
	m.repository.EXPECT().URI().AnyTimes()
}

func TestTaskRunOpts_Execute(t *testing.T) {
	const (
		inGroupName = "my-task"
		mockRepoURI = "uri/repo"
		tag         = "tag"
	)
	defaultBuildArguments := exec.BuildArguments{
		Context:  filepath.Dir(defaultDockerfilePath),
		ImageTag: imageTagLatest,
	}

	testCases := map[string]struct {
		inImage      string
		inTag        string
		inFollow     bool
		inCommand    string
		inEntryPoint string

		inEnv string

		setupMocks func(m runTaskMocks)

		wantedError error
	}{
		"check if default cluster exists if deploying to default cluster": {
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.defaultClusterGetter.EXPECT().HasDefaultCluster().Return(true, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
			},
		},
		"do not check for default cluster if deploying to environment": {
			inEnv: "test",
			setupMocks: func(m runTaskMocks) {
				m.defaultClusterGetter.EXPECT().HasDefaultCluster().Times(0)
				m.store.EXPECT().
					GetEnvironment(gomock.Any(), "test").
					Return(&config.Environment{
						ExecutionRoleARN: "env execution role",
					}, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
			},
		},
		"error deploying resources": {
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{},
					EntryPoint: []string{},
				}).Return(errors.New("error deploying"))
				mockHasDefaultCluster(m)
			},
			wantedError: errors.New("provision resources for task my-task: error deploying"),
		},
		"error updating resources": {
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{},
					EntryPoint: []string{},
				}).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI)
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "uri/repo:latest",
					Command:    []string{},
					EntryPoint: []string{},
				}).Times(1).Return(errors.New("error updating"))
				mockHasDefaultCluster(m)
			},
			wantedError: errors.New("update resources for task my-task: error updating"),
		},
		"error running tasks": {
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().Return(nil, errors.New("error running"))
				mockHasDefaultCluster(m)
			},
			wantedError: errors.New("run task my-task: error running"),
		},
		"deploy with execution role option if env is not empty": {
			inEnv: "test",
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), "test").
					Return(&config.Environment{
						ExecutionRoleARN: "env execution role",
					}, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any(), gomock.Len(1)).AnyTimes() // NOTE: matching length because gomock is unable to match function arguments.
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
				m.defaultClusterGetter.EXPECT().HasDefaultCluster().Times(0)
			},
		},
		"deploy without execution role option if env is empty": {
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any(), gomock.Len(0)).AnyTimes() // NOTE: matching length because gomock is unable to match function arguments.
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
				mockHasDefaultCluster(m)
			},
		},
		"append 'latest' to image tag": {
			inTag: tag,
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(
					&exec.BuildArguments{
						Context:        filepath.Dir(defaultDockerfilePath),
						ImageTag:       imageTagLatest,
						AdditionalTags: []string{tag},
					}),
				)
				m.repository.EXPECT().URI().AnyTimes()
				m.runner.EXPECT().Run().AnyTimes()
				mockHasDefaultCluster(m)
			},
		},
		"update image to task resource if image is not provided": {
			inCommand:    `/bin/sh -c "curl $ECS_CONTAINER_METADATA_URI_V4"`,
			inEntryPoint: `exec "some command"`,
			setupMocks: func(m runTaskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{"/bin/sh", "-c", "curl $ECS_CONTAINER_METADATA_URI_V4"},
					EntryPoint: []string{"exec", "some command"},
				}).Times(1).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI)
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "uri/repo:latest",
					Command:    []string{"/bin/sh", "-c", "curl $ECS_CONTAINER_METADATA_URI_V4"},
					EntryPoint: []string{"exec", "some command"},
				}).Times(1).Return(nil)
				m.runner.EXPECT().Run().AnyTimes()
				mockHasDefaultCluster(m)
			},
		},
		"fail to get ENI information for some tasks": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:      "eni-1",
					},
					{
						TaskARN: "task-2",
					},
					{
						TaskARN: "task-3",
					},
				}, &task.ErrENIInfoNotFoundForTasks{
					Errors: []*ecs.ErrTaskENIInfoNotFound{
						{
							MissingField: "attachment",
							TaskARN:      "task-2",
						},
						{
							MissingField: "attachment",
							TaskARN:      "task-3",
						},
					},
				})
				m.publicIPGetter.EXPECT().PublicIP("eni-1").Return("1.2.3", nil)
				mockHasDefaultCluster(m)
				mockRepositoryAnytime(m)
			},
			// we dont want to return error; instead, we just want to print logs that listing the tasks for which we cannot
			// find the ENI information
		},
		"fail to get public ips": {
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:      "eni-1",
					},
				}, nil)
				m.publicIPGetter.EXPECT().PublicIP("eni-1").Return("", errors.New("some error"))
				mockHasDefaultCluster(m)
				mockRepositoryAnytime(m)
			},
			wantedError: errors.New("get public ip for task task-1: some error"),
		},
		"fail to write events": {
			inFollow: true,
			inImage:  "image",
			setupMocks: func(m runTaskMocks) {
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:      "eni-1",
					},
				}, nil)
				m.publicIPGetter.EXPECT().PublicIP("eni-1").Return("1.2.3", nil)
				m.eventsWriter.EXPECT().WriteEventsUntilStopped().Times(1).
					Return(errors.New("error writing events"))
				mockHasDefaultCluster(m)
			},
			wantedError: errors.New("write events: error writing events"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := runTaskMocks{
				deployer:             mocks.NewMocktaskDeployer(ctrl),
				repository:           mocks.NewMockrepositoryService(ctrl),
				runner:               mocks.NewMocktaskRunner(ctrl),
				store:                mocks.NewMockstore(ctrl),
				eventsWriter:         mocks.NewMockeventsWriter(ctrl),
				defaultClusterGetter: mocks.NewMockdefaultClusterGetter(ctrl),
				publicIPGetter:       mocks.NewMockpublicIPGetter(ctrl),
			}
			tc.setupMocks(mocks)

			opts := &runTaskOpts{
				runTaskVars: runTaskVars{
					groupName: inGroupName,

					image:      tc.inImage,
					imageTag:   tc.inTag,
					env:        tc.inEnv,
					follow:     tc.inFollow,
					command:    tc.inCommand,
					entrypoint: tc.inEntryPoint,
				},
				spinner: &mockSpinner{},
				store:   mocks.store,
			}
			opts.configureRuntimeOpts = func() error {
				opts.runner = mocks.runner
				opts.deployer = mocks.deployer
				opts.defaultClusterGetter = mocks.defaultClusterGetter
				opts.publicIPGetter = mocks.publicIPGetter
				return nil
			}
			opts.configureRepository = func() error {
				opts.repository = mocks.repository
				return nil
			}
			opts.configureEventsWriter = func(tasks []*task.Task) {
				opts.eventsWriter = mocks.eventsWriter
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
