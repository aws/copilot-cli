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
	inNum    int8
	inCPU    int16
	inMemory int16
}

func TestTaskRunOpts_Validate(t *testing.T) {
	defaultOpts := basicOpts{
		inNum:    1,
		inCPU:    256,
		inMemory: 512,
	}

	testCases := map[string]struct {
		basicOpts

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
				inNum:    -1,
				inCPU:    256,
				inMemory: 512,
			},
			wantedError: errNumNotPositive,
		},
		"invalid number of CPU units": {
			basicOpts: basicOpts{
				inNum:    1,
				inCPU:    -15,
				inMemory: 512,
			},
			wantedError: errCpuNotPositive,
		},
		"invalid memory": {
			basicOpts: basicOpts{
				inNum:    1,
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
			inEnv: "dev",

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
			inEnv: "dev",

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

			wantedError: errors.New("neither subnet nor Security Groups should be specified if environment is specified"),
		},
		"both environment and security groups specified": {
			basicOpts: defaultOpts,

			inEnv:            "test",
			inSecurityGroups: []string{"security group id1", "securty group id2"},

			wantedError: errors.New("neither subnet nor Security Groups should be specified if environment is specified"),
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
					num:            tc.inNum,
					cpu:            tc.inCPU,
					memory:         tc.inMemory,
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
