// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type basicOpts struct {
	inCount  int64
	inCPU    int
	inMemory int
}

var defaultOpts = basicOpts{
	inCount:  1,
	inCPU:    256,
	inMemory: 512,
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
			wantedError: fmt.Errorf("get application: couldn't find an application named my-app in account 115 and region us-east-1"),
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
			wantedError: errors.New("get environment: couldn't find environment dev in the application my-app"),
		},
		"no workspace": {
			basicOpts: defaultOpts,

			inEnv:       "test",
			wantedError: errNoAppInWorkspace,
		},
		"both environment and subnet ID specified": {
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
				require.Nil(t, err)
			}
		})
	}
}

func TestTaskRunOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		basicOpts

		inName string

		inEnv   string
		appName string

		mockSel    func(m *mocks.MockappEnvWithNoneSelector)
		mockPrompt func(m *mocks.Mockprompter)

		wantedError error
		wantedEnv   string
		wantedName  string
	}{
		"prompt for env": {
			basicOpts: defaultOpts,

			inName:  "my-task",
			appName: "my-app",

			mockSel: func(m *mocks.MockappEnvWithNoneSelector) {
				m.EXPECT().EnvironmentWithNone(fmtTaskRunEnvPrompt, gomock.Any(), "my-app").Return("test", nil)
			},

			wantedEnv: "test",
		},
		"don't prompt if env is provided": {
			basicOpts: defaultOpts,

			inName:  "my-task",
			inEnv:   "test",
			appName: "my-app",

			mockSel: func(m *mocks.MockappEnvWithNoneSelector) {
				m.EXPECT().EnvironmentWithNone(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: "test",
		},
		"don't prompt if no workspace": {
			basicOpts: defaultOpts,

			inName: "my-task",

			mockSel: func(m *mocks.MockappEnvWithNoneSelector) {
				m.EXPECT().EnvironmentWithNone(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedEnv: config.EnvNameNone,
		},
		"prompt for task family name": {
			basicOpts: defaultOpts,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(fmtTaskRunGroupNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("my-task", nil)
			},

			wantedName: "my-task",
		},
		"error getting task group name": {
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("error getting task group name"))
			},
			wantedError: errors.New("prompt get task group name: error getting task group name"),
		},
		"error selecting environment": {
			basicOpts: defaultOpts,

			inName:  "my-task",
			appName: "my-app",

			mockSel: func(m *mocks.MockappEnvWithNoneSelector) {
				m.EXPECT().EnvironmentWithNone(fmtTaskRunEnvPrompt, gomock.Any(), gomock.Any()).Return(config.EnvNameNone, fmt.Errorf("error selecting environment"))
			},

			wantedError: errors.New("ask for environment: error selecting environment"),
			wantedEnv:   config.EnvNameNone,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockappEnvWithNoneSelector(ctrl)
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
					count:  tc.inCount,
					cpu:    tc.inCPU,
					memory: tc.inMemory,

					groupName: tc.inName,
					env:       tc.inEnv,
				},
				sel: mockSel,
			}

			err := opts.Ask()

			if tc.wantedError == nil {
				require.Nil(t, err)
				if tc.wantedEnv != "" {
					require.Equal(t, tc.wantedEnv, opts.env)
				}
				if tc.wantedName != "" {
					require.Equal(t, tc.wantedName, opts.groupName)
				}
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}

func TestTaskRunOpts_getNetworkConfig(t *testing.T) {
	testCases := map[string]struct {
		inSubnets        []string
		inSecurityGroups []string

		appName string
		env     string

		mockVPC func(m *mocks.MockvpcService)

		wantedError          error
		wantedSubnets        []string
		wantedSecurityGroups []string
	}{
		"don't get default subnet IDs if they are provided": {
			env:       config.EnvNameNone,
			inSubnets: []string{"subnet-1", "subnet-3"},

			mockVPC: func(m *mocks.MockvpcService) {
				m.EXPECT().GetSubnetIDs(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().GetSecurityGroups(gomock.Any(), gomock.Any()).AnyTimes()
			},

			wantedSubnets: []string{"subnet-1", "subnet-3"},
		},
		"don't get default security groups if they are provided": {
			env:              config.EnvNameNone,
			inSecurityGroups: []string{"sg-1", "sg-3"},

			mockVPC: func(m *mocks.MockvpcService) {
				m.EXPECT().GetSubnetIDs(gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().GetSecurityGroups(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedSecurityGroups: []string{"sg-1", "sg-3"},
		},
		"error getting subnets from app env": {
			appName: "my-app",
			env:     "test",

			mockVPC: func(m *mocks.MockvpcService) {
				m.EXPECT().GetSubnetIDs("my-app", "test").Return(nil, errors.New("error")).Times(1)
				m.EXPECT().GetSecurityGroups(gomock.Any(), gomock.Any()).AnyTimes()
			},

			wantedError: errors.New("get subnet IDs from environment test: error"),
		},
		"error getting security groups from app env": {
			appName: "my-app",
			env:     "test",

			mockVPC: func(m *mocks.MockvpcService) {
				m.EXPECT().GetSubnetIDs(gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().GetSecurityGroups("my-app", "test").Return(nil, errors.New("error")).Times(1)
			},

			wantedError: errors.New("get security groups from environment test: error"),
		},
		"get subnets and security-groups from app env": {
			appName: "my-app",
			env:     "test",

			mockVPC: func(m *mocks.MockvpcService) {
				m.EXPECT().GetSubnetIDs("my-app", "test").Return([]string{"subnet-3", "subnet-4"}, nil).Times(1)
				m.EXPECT().GetSecurityGroups("my-app", "test").Return([]string{"sg-3", "sg-4"}, nil).Times(1)
			},

			wantedSubnets:        []string{"subnet-3", "subnet-4"},
			wantedSecurityGroups: []string{"sg-3", "sg-4"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVpc := mocks.NewMockvpcService(ctrl)

			if tc.mockVPC != nil {
				tc.mockVPC(mockVpc)
			}

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.appName,
					},
					env:            tc.env,
					subnets:        tc.inSubnets,
					securityGroups: tc.inSecurityGroups,
				},
				vpcGetter: mockVpc,
			}

			err := opts.getNetworkConfig()

			if tc.wantedError == nil {
				require.Nil(t, err)
				if tc.wantedSubnets != nil {
					require.Equal(t, tc.wantedSubnets, opts.subnets)
				}
				if tc.wantedSecurityGroups != nil {
					require.Equal(t, tc.wantedSecurityGroups, opts.securityGroups)
				}
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}

func TestTaskRunOpts_pushToECRRepo(t *testing.T) {
	testCases := map[string]struct {
		inGroupName      string
		inDockerfilePath string
		inImageTag       string

		mockEcr    func(m *mocks.MockecrService)
		mockDocker func(m *mocks.MockdockerService)

		wantedError error
		wantedUri   string
	}{
		"success": {
			inGroupName:      "my-task",
			inDockerfilePath: "./Dockerfile",
			inImageTag:       "0.1",

			mockEcr: func(m *mocks.MockecrService) {
				m.EXPECT().GetRepository("copilot-my-task").Return("aws.ecr.my-task", nil).Times(1)
				m.EXPECT().GetECRAuth().Return(ecr.Auth{}, nil).Times(1)
			},
			mockDocker: func(m *mocks.MockdockerService) {
				m.EXPECT().Build("aws.ecr.my-task", "0.1", "./Dockerfile").Return(nil).Times(1)
				m.EXPECT().Login("aws.ecr.my-task", gomock.Any(), gomock.Any()).Return(nil).Times(1)
				m.EXPECT().Push("aws.ecr.my-task", "0.1").Return(nil).Times(1)
			},
			wantedUri: "aws.ecr.my-task",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEcr := mocks.NewMockecrService(ctrl)
			mockDocker := mocks.NewMockdockerService(ctrl)

			if tc.mockEcr != nil {
				tc.mockEcr(mockEcr)
			}
			if tc.mockDocker != nil {
				tc.mockDocker(mockDocker)
			}

			opts := runTaskOpts{
				runTaskVars: runTaskVars{
					groupName:      tc.inGroupName,
					dockerfilePath: tc.inDockerfilePath,
					imageTag:       tc.inImageTag,
				},
				ecrGetter: mockEcr,
				docker:    mockDocker,
			}

			uri, err := opts.pushToECRRepo()

			if tc.wantedError == nil {
				require.Nil(t, err)
				require.Equal(t, tc.wantedUri, uri)
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}
