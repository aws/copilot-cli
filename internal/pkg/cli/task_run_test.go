// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/ecs"
	ecsMocks "github.com/aws/copilot-cli/internal/pkg/ecs/mocks"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
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

		inImage                 string
		inDockerfilePath        string
		inDockerfileContextPath string

		inTaskRole string

		inEnv            string
		inCluster        string
		inSubnets        []string
		inSecurityGroups []string

		inEnvVars    map[string]string
		inSecrets    map[string]string
		inCommand    string
		inEntryPoint string
		inOS         string
		inArch       string

		inDefault               bool
		inGenerateCommandTarget string

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
			inSecrets: map[string]string{
				"quiet": "barky doggo",
			},
			inCommand:    "echo hello world",
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
		"invalid with os but not arch": {
			basicOpts:   defaultOpts,
			inOS:        "WINDOWS_SERVER_2019_CORE",
			wantedError: errors.New("must specify either both `--platform-os` and `--platform-arch` or neither"),
		},
		"invalid with arch but not os": {
			basicOpts:   defaultOpts,
			inArch:      "X86_64",
			wantedError: errors.New("must specify either both `--platform-os` and `--platform-arch` or neither"),
		},
		"invalid platform": {
			basicOpts:   defaultOpts,
			inOS:        "OStrich",
			inArch:      "MAD666",
			wantedError: errors.New("platform OSTRICH/MAD666 is invalid; valid platforms are: WINDOWS_SERVER_2019_CORE/X86_64, WINDOWS_SERVER_2019_FULL/X86_64, LINUX/X86_64 and LINUX/ARM64"),
		},
		"uppercase any lowercase before validating": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    1024,
				inMemory: 2048,
			},
			inOS:        "windows_server_2019_core",
			inArch:      "x86_64",
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
		"invalid number of CPU units for Windows task": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    260,
				inMemory: 512,
			},
			inOS:        "WINDOWS_SERVER_2019_CORE",
			inArch:      "X86_64",
			wantedError: errors.New("CPU is 260, but it must be at least 1024 for a Windows-based task"),
		},
		"invalid memory": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    256,
				inMemory: -1024,
			},
			wantedError: errMemNotPositive,
		},
		"invalid memory for Windows task": {
			basicOpts: basicOpts{
				inCount:  1,
				inCPU:    1024,
				inMemory: 2000,
			},
			inOS:        "WINDOWS_SERVER_2019_CORE",
			inArch:      "X86_64",
			wantedError: errors.New("memory is 2000, but it must be at least 2048 for a Windows-based task"),
		},
		"both build context and image name specified": {
			basicOpts: defaultOpts,

			inImage:                 "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			inDockerfileContextPath: "../../other",

			wantedError: errors.New("cannot specify both `--image` and `--build-context`"),
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

			wantedError: errors.New("invalid `--dockerfile` path: open world/hello/Dockerfile: file does not exist"),
		},
		"invalid build context path": {
			basicOpts: defaultOpts,

			inDockerfileContextPath: "world/hello/Dockerfile",

			wantedError: errors.New("invalid `--build-context` path: open world/hello/Dockerfile: file does not exist"),
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
		"both application and subnets specified": {
			basicOpts: defaultOpts,

			appName:   "my-app",
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("cannot specify both `--subnets` and `--app`"),
		},
		"both default and subnets specified": {
			basicOpts: defaultOpts,

			inDefault: true,
			inSubnets: []string{"subnet id"},

			wantedError: errors.New("cannot specify both `--subnets` and `--default`"),
		},
		"both cluster and default specified": {
			basicOpts: defaultOpts,

			inDefault: true,
			inCluster: "special-cluster",

			wantedError: errors.New("cannot specify both `--default` and `--cluster`"),
		},
		"both cluster and application specified": {
			basicOpts: defaultOpts,

			inCluster: "special-cluster",
			appName:   "my-app",

			wantedError: errors.New("cannot specify both `--app` and `--cluster`"),
		},
		"both cluster and environment specified": {
			basicOpts: defaultOpts,

			inCluster: "special-cluster",
			inEnv:     "my-env",

			wantedError: errors.New("cannot specify both `--env` and `--cluster`"),
		},
		"generate-cmd specified with another flag": {
			basicOpts: defaultOpts,

			inGenerateCommandTarget: "cluster/service", // nFlag is set to 2.

			wantedError: errors.New("cannot specify `--generate-cmd` with any other flag"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					appName:                     tc.appName,
					count:                       tc.inCount,
					cpu:                         tc.inCPU,
					memory:                      tc.inMemory,
					groupName:                   tc.inName,
					image:                       tc.inImage,
					env:                         tc.inEnv,
					taskRole:                    tc.inTaskRole,
					cluster:                     tc.inCluster,
					subnets:                     tc.inSubnets,
					securityGroups:              tc.inSecurityGroups,
					dockerfilePath:              tc.inDockerfilePath,
					dockerfileContextPath:       tc.inDockerfileContextPath,
					envVars:                     tc.inEnvVars,
					secrets:                     tc.inSecrets,
					command:                     tc.inCommand,
					entrypoint:                  tc.inEntryPoint,
					useDefaultSubnetsAndCluster: tc.inDefault,
					generateCommandTarget:       tc.inGenerateCommandTarget,
					os:                          tc.inOS,
					arch:                        tc.inArch,
				},
				isDockerfileSet: tc.isDockerfileSet,
				nFlag:           2,

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

		inCluster        string
		inSubnets        []string
		inSecurityGroups []string

		inDefault                  bool
		inEnv                      string
		appName                    string
		inSecrets                  map[string]string
		inSsmParamSecrets          map[string]string
		inSecretsManagerSecrets    map[string]string
		inAcknowledgeSecretsAccess bool
		inExecutionRole            string

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
		"don't prompt for app if cluster is specified": {
			inCluster: "cluster-1",
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			mockSel: func(m *mocks.MockappEnvSelector) {
				m.EXPECT().Application(taskRunAppPrompt, gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Environment(taskRunEnvPrompt, gomock.Any(), gomock.Any(), appEnvOptionNone).Times(0)
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
		"don't prompt for env if cluster is specified": {
			inCluster: "cluster-1",
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
		"When secrets are provided without app and env leads to a secret access permission prompt": {
			inSecrets: map[string]string{
				"quiet": "shh",
			},
			inSsmParamSecrets: map[string]string{
				"quiet": "shh",
			},
			inSecretsManagerSecrets: map[string]string{
				"quiet": "shh",
			},
			inCluster: "cluster-1",
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(taskSecretsPermissionPrompt, taskSecretsPermissionPromptHelp).Return(true, nil)
			},
		},
		"secret access permission prompt is skipped when acknowledge-secret-access flag is provided": {
			inSecrets: map[string]string{
				"quiet": "shh",
			},
			inCluster:                  "cluster-1",
			inAcknowledgeSecretsAccess: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(taskSecretsPermissionPrompt, taskSecretsPermissionPromptHelp).Times(0)
			},
		},
		"secret access permission prompt is skipped when execution-role is provided": {
			inSecrets: map[string]string{
				"quiet": "shh",
			},
			inCluster:       "cluster-1",
			inExecutionRole: "test-role",
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(taskSecretsPermissionPrompt, taskSecretsPermissionPromptHelp).Times(0)
			},
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
					appName:                     tc.appName,
					groupName:                   tc.inName,
					env:                         tc.inEnv,
					useDefaultSubnetsAndCluster: tc.inDefault,
					subnets:                     tc.inSubnets,
					securityGroups:              tc.inSecurityGroups,
					cluster:                     tc.inCluster,
					acknowledgeSecretsAccess:    tc.inAcknowledgeSecretsAccess,
					secrets:                     tc.inSecrets,
					executionRole:               tc.inExecutionRole,
				},
				sel:                   mockSel,
				prompt:                mockPrompter,
				secretsManagerSecrets: tc.inSecretsManagerSecrets,
				ssmParamSecrets:       tc.inSsmParamSecrets,
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
	provider             *mocks.MocksessionProvider
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
	defaultBuildArguments := dockerengine.BuildArguments{
		Context: filepath.Dir(defaultDockerfilePath),
		Tags:    []string{imageTagLatest},
	}

	testCases := map[string]struct {
		inSecrets    map[string]string
		inImage      string
		inTag        string
		inDockerCtx  string
		inFollow     bool
		inCommand    string
		inEntryPoint string

		inEnv string

		setupMocks func(m runTaskMocks)

		wantedError error
	}{
		"check if default cluster exists if deploying to default cluster": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
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
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any())
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
			},
		},
		"error deploying resources": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
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
		"error getting repo URI": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{},
					EntryPoint: []string{},
				}).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI, errors.New("some error"))
				mockHasDefaultCluster(m)
			},
			wantedError: errors.New("get ECR repository URI: some error"),
		},
		"error updating resources": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{},
					EntryPoint: []string{},
				}).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI, nil)
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
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
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
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any())
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any(), gomock.Len(1)).AnyTimes() // NOTE: matching length because gomock is unable to match function arguments.
				mockRepositoryAnytime(m)
				m.runner.EXPECT().Run().AnyTimes()
				m.defaultClusterGetter.EXPECT().HasDefaultCluster().Times(0)
			},
		},
		"deploy without execution role option if env is empty": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
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
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(
					&dockerengine.BuildArguments{
						Context: filepath.Dir(defaultDockerfilePath),
						Tags:    []string{imageTagLatest, tag},
					}),
				)
				m.repository.EXPECT().URI().AnyTimes()
				m.runner.EXPECT().Run().AnyTimes()
				mockHasDefaultCluster(m)
			},
		},
		"should use provided docker build context instead of dockerfile path": {
			inDockerCtx: "../../other",
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(
					&dockerengine.BuildArguments{
						Context: "../../other",
						Tags:    []string{imageTagLatest},
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
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.deployer.EXPECT().DeployTask(gomock.Any(), &deploy.CreateTaskResourcesInput{
					Name:       inGroupName,
					Image:      "",
					Command:    []string{"/bin/sh", "-c", "curl $ECS_CONTAINER_METADATA_URI_V4"},
					EntryPoint: []string{"exec", "some command"},
				}).Times(1).Return(nil)
				m.repository.EXPECT().BuildAndPush(gomock.Any(), gomock.Eq(&defaultBuildArguments))
				m.repository.EXPECT().URI().Return(mockRepoURI, nil)
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
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:     "eni-1",
					},
					{
						TaskARN: "task-2",
					},
					{
						TaskARN: "task-3",
					},
				}, nil)
				m.publicIPGetter.EXPECT().PublicIP("eni-1").Return("1.2.3", nil)
				mockHasDefaultCluster(m)
				mockRepositoryAnytime(m)
			},
		},
		"fail to get public ips": {
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:     "eni-1",
					},
				}, nil)
				m.publicIPGetter.EXPECT().PublicIP("eni-1").Return("", errors.New("some error"))
				mockHasDefaultCluster(m)
				mockRepositoryAnytime(m)
			},
			// wantedError is nil because we will just not show the IP address if we can't instead of erroring out.
		},
		"fail to write events": {
			inFollow: true,
			inImage:  "image",
			setupMocks: func(m runTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.deployer.EXPECT().DeployTask(gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run().Return([]*task.Task{
					{
						TaskARN: "task-1",
						ENI:     "eni-1",
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
				provider:             mocks.NewMocksessionProvider(ctrl),
			}
			tc.setupMocks(mocks)

			opts := &runTaskOpts{
				runTaskVars: runTaskVars{
					groupName: inGroupName,

					image:                 tc.inImage,
					imageTag:              tc.inTag,
					dockerfileContextPath: tc.inDockerCtx,

					env:        tc.inEnv,
					follow:     tc.inFollow,
					secrets:    tc.inSecrets,
					command:    tc.inCommand,
					entrypoint: tc.inEntryPoint,
				},
				spinner:  &mockSpinner{},
				store:    mocks.store,
				provider: mocks.provider,
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

type mockRunTaskRequester struct {
	mockRunTaskRequestFromECSService func(client ecs.ECSServiceDescriber, cluster string, service string) (*ecs.RunTaskRequest, error)
	mockRunTaskRequestFromService    func(client ecs.ServiceDescriber, app, env, svc string) (*ecs.RunTaskRequest, error)
	mockRunTaskRequestFromJob        func(client ecs.JobDescriber, app, env, job string) (*ecs.RunTaskRequest, error)
}

type taskRunMocks struct {
	store    *mocks.Mockstore
	provider *mocks.MocksessionProvider
}

func TestTaskRunOpts_runTaskCommand(t *testing.T) {
	wantedCommand := ecs.RunTaskRequest{}

	testCases := map[string]struct {
		inGenerateCommandTarget string

		setUpMocks           func(m *taskRunMocks)
		mockRunTaskRequester mockRunTaskRequester

		wantedCommand *ecs.RunTaskRequest
		wantedError   error
	}{
		"should generate a command given an service ARN": {
			inGenerateCommandTarget: "arn:aws:ecs:us-east-1:123456789012:service/crowded-cluster/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.provider.EXPECT().Default()
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromECSService: func(client ecs.ECSServiceDescriber, cluster string, service string) (*ecs.RunTaskRequest, error) {
					return &wantedCommand, nil
				},
			},
			wantedCommand: &wantedCommand,
		},
		"fail to generate a command given an service ARN": {
			inGenerateCommandTarget: "arn:aws:ecs:us-east-1:123456789012:service/crowded-cluster/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.provider.EXPECT().Default()
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromECSService: func(client ecs.ECSServiceDescriber, cluster string, service string) (*ecs.RunTaskRequest, error) {
					return nil, errors.New("some error")
				},
			},
			wantedError: fmt.Errorf("generate task run command from ECS service crowded-cluster/good-service: some error"),
		},
		"should generate a command given a cluster/service target": {
			inGenerateCommandTarget: "crowded-cluster/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.provider.EXPECT().Default()
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromECSService: func(client ecs.ECSServiceDescriber, cluster string, service string) (*ecs.RunTaskRequest, error) {
					return &wantedCommand, nil
				},
			},
			wantedCommand: &wantedCommand,
		},
		"fail to generate a command given a cluster/service target": {
			inGenerateCommandTarget: "crowded-cluster/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.provider.EXPECT().Default()
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromECSService: func(client ecs.ECSServiceDescriber, cluster string, service string) (*ecs.RunTaskRequest, error) {
					return nil, errors.New("some error")
				},
			},
			wantedError: fmt.Errorf("generate task run command from ECS service crowded-cluster/good-service: some error"),
		},
		"should generate a command given an app/env/svc target": {
			inGenerateCommandTarget: "good-app/good-env/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "good-service").Return(nil, &config.ErrNoSuchJob{})
				m.store.EXPECT().GetService("good-app", "good-service").Return(&config.Workload{}, nil)
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromService: func(client ecs.ServiceDescriber, app, env, svc string) (*ecs.RunTaskRequest, error) {
					return &wantedCommand, nil
				},
			},
			wantedCommand: &wantedCommand,
		},
		"fail to generate a command given an app/env/svc target": {
			inGenerateCommandTarget: "good-app/good-env/good-service",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "good-service").Return(nil, &config.ErrNoSuchJob{})
				m.store.EXPECT().GetService("good-app", "good-service").Return(&config.Workload{}, nil)
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromService: func(client ecs.ServiceDescriber, app, env, svc string) (*ecs.RunTaskRequest, error) {
					return nil, errors.New("some error")
				},
			},
			wantedError: fmt.Errorf("generate task run command from service good-service of application good-app deployed in environment good-env: some error"),
		},
		"should generate a command given an app/env/job target": {
			inGenerateCommandTarget: "good-app/good-env/good-job",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "good-job").Return(&config.Workload{}, nil)
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromJob: func(client ecs.JobDescriber, app, env, svc string) (*ecs.RunTaskRequest, error) {
					return &wantedCommand, nil
				},
			},
			wantedCommand: &wantedCommand,
		},
		"fail to generate a command given an app/env/job target": {
			inGenerateCommandTarget: "good-app/good-env/good-job",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "good-job").Return(&config.Workload{}, nil)
			},
			mockRunTaskRequester: mockRunTaskRequester{
				mockRunTaskRequestFromJob: func(client ecs.JobDescriber, app, env, svc string) (*ecs.RunTaskRequest, error) {
					return nil, errors.New("some error")
				},
			},
			wantedError: fmt.Errorf("generate task run command from job good-job of application good-app deployed in environment good-env: some error"),
		},
		"fail to determine if the workload is a job given an app/env/workload target": {
			inGenerateCommandTarget: "good-app/good-env/bad-workload",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "bad-workload").Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("determine whether workload bad-workload is a job: some error"),
		},
		"fail to determine if the workload is a service given an app/env/workload target": {
			inGenerateCommandTarget: "good-app/good-env/bad-workload",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "bad-workload").Return(nil, &config.ErrNoSuchJob{})
				m.store.EXPECT().GetService("good-app", "bad-workload").Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("determine whether workload bad-workload is a service: some error"),
		},
		"workload is neither a job nor a service": {
			inGenerateCommandTarget: "good-app/good-env/bad-workload",
			setUpMocks: func(m *taskRunMocks) {
				m.store.EXPECT().GetEnvironment("good-app", "good-env").Return(&config.Environment{
					ManagerRoleARN: "mock-role",
					Region:         "mock-region",
				}, nil)
				m.provider.EXPECT().FromRole("mock-role", "mock-region")
				m.store.EXPECT().GetJob("good-app", "bad-workload").Return(nil, &config.ErrNoSuchJob{})
				m.store.EXPECT().GetService("good-app", "bad-workload").Return(nil, &config.ErrNoSuchService{})
			},
			wantedError: fmt.Errorf("workload bad-workload is neither a service nor a job"),
		},
		"invalid input": {
			inGenerateCommandTarget: "invalid/illegal/not-good/input/is/bad",
			setUpMocks:              func(m *taskRunMocks) {},
			wantedError:             errors.New("invalid input to --generate-cmd: must be of format <cluster>/<service> or <app>/<env>/<workload>"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &taskRunMocks{
				store:    mocks.NewMockstore(ctrl),
				provider: mocks.NewMocksessionProvider(ctrl),
			}
			if tc.setUpMocks != nil {
				tc.setUpMocks(m)
			}
			opts := &runTaskOpts{
				runTaskVars: runTaskVars{
					generateCommandTarget: tc.inGenerateCommandTarget,
				},
				store:    m.store,
				provider: m.provider,

				configureECSServiceDescriber: func(session *session.Session) ecs.ECSServiceDescriber {
					return ecsMocks.NewMockECSServiceDescriber(ctrl)
				},
				configureJobDescriber: func(session *session.Session) ecs.JobDescriber {
					return ecsMocks.NewMockJobDescriber(ctrl)
				},
				configureServiceDescriber: func(session *session.Session) ecs.ServiceDescriber {
					return ecsMocks.NewMockServiceDescriber(ctrl)
				},

				runTaskRequestFromECSService: tc.mockRunTaskRequester.mockRunTaskRequestFromECSService,
				runTaskRequestFromService:    tc.mockRunTaskRequester.mockRunTaskRequestFromService,
				runTaskRequestFromJob:        tc.mockRunTaskRequester.mockRunTaskRequestFromJob,
			}

			got, err := opts.runTaskCommand()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedCommand, got)
			}
		})
	}
}
