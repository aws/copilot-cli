// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

type basicOpts struct {
	inCount  int8
	inCPU    int16
	inMemory int16
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
		inSubnet         string
		inSecurityGroups []string

		inEnvVars  map[string]string
		inCommands []string

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
			inCommands: []string{"echo hello world"},

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

			inSubnet:         "subnet-10d938jds",
			inSecurityGroups: []string{"sg-0d9sjdk", "sg-d33kds99"},

			inEnvVars: map[string]string{
				"NAME": "pj",
				"ENV":  "dev",
			},
			inCommands: []string{"echo hello world"},

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
		"both environment and subnet id specified": {
			basicOpts: defaultOpts,

			inEnv:    "test",
			inSubnet: "subnet id",

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
					count:            tc.inCount,
					cpu:            tc.inCPU,
					memory:         tc.inMemory,
					groupName:      tc.inName,
					image:          tc.inImage,
					env:            tc.inEnv,
					taskRole:       tc.inTaskRole,
					subnet:         tc.inSubnet,
					securityGroups: tc.inSecurityGroups,
					dockerfilePath: tc.inDockerfilePath,
					envVars:        tc.inEnvVars,
					commands:       tc.inCommands,
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

			inName: "my-task",
			appName: "my-app",

			mockSel: func(m *mocks.MockappEnvWithNoneSelector) {
				m.EXPECT().EnvironmentWithNone(fmtTaskRunEnvPrompt, gomock.Any(), gomock.Any()).Return(config.EnvNameNone, fmt.Errorf("error selecting environment"))
			},

			wantedError: errors.New("ask for environment: error selecting environment"),
			wantedEnv: config.EnvNameNone,
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
					count:    tc.inCount,
					cpu:    tc.inCPU,
					memory: tc.inMemory,

					groupName: tc.inName,
					env:       tc.inEnv,
				},
				sel:   mockSel,
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
