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
	inNum              	uint8
	inCPU              	uint16
	inMemory           	uint16
}

func TestTaskRunOpts_Validate(t *testing.T) {
	defaultOpts := basicOpts{
		inNum: 1,
		inCPU: 256,
		inMemory: 512,
	}

	testCases := map[string]struct {
		basicOpts

		inImage            	string
		inDockerfilePath   	string

		inTaskRole         	string

		inApp  				string

		inEnv              	string
		inSubnetID         	string
		inSecurityGroupIDs 	[]string

		inEnvVars          	map[string]string
		inCommands         	string

		appName				string

		mockStore			func(m *mocks.Mockstore)
		mockFileSystem func(mockFS afero.Fs)

		wantedError			error
	}{
		"valid with no flag": {
			basicOpts:   defaultOpts,
			wantedError: nil,
		},
		"valid image and env": {
			basicOpts: defaultOpts,

			inImage: "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			inTaskRole: "exec-role",

			inEnv: "dev",

			inEnvVars: map[string]string{
				"NAME": "my-app",
				"ENV":	"dev",
			},
			inCommands: "echo \"docker commands\"",

			appName: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "dev").Return(&config.Environment{
					App: "my-app",
					Name: "dev",
				}, nil)
			},

			wantedError: nil,
		},
		"valid without image and env": {
			basicOpts: defaultOpts,

			inDockerfilePath: "hello/world/Dockerfile",
			inTaskRole: "exec-role",

			inSubnetID: "subnet-10d938jds",
			inSecurityGroupIDs: []string{"sg-0d9sjdk", "sg-d33kds99"},

			inEnvVars: map[string]string{
				"NAME": "pj",
				"ENV":	"dev",
			},
			inCommands: "echo \"docker commands\"",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello/world", 0755)
				afero.WriteFile(mockFS, "hello/world/Dockerfile", []byte("FROM nginx"), 0644)
			},
			wantedError: nil,
		},
		"invalid number of tasks": {
			basicOpts: basicOpts{
				inNum: 0,
				inCPU: 256,
				inMemory: 512,
			},
			wantedError: errors.New("number of tasks must be positive"),
		},
		"invalid number of CPU units": {
			basicOpts: basicOpts{
				inNum: 1,
				inCPU: 0,
				inMemory: 512,
			},
			wantedError: errors.New("CPU units must be positive"),
		},
		"invalid memory": {
			basicOpts: basicOpts{
				inNum: 1,
				inCPU: 256,
				inMemory: 0,
			},
			wantedError: errors.New("memory must be positive"),
		},
		"both dockerfile and image name specified": {
			basicOpts: defaultOpts,

			inImage: "113459295.dkr.ecr.ap-northeast-1.amazonaws.com/my-app",
			inDockerfilePath: "hello/world/Dockerfile",

			wantedError: errors.New("cannot specify both image and dockerfile path"),
		},
		"invalid dockerfile path": {
			basicOpts: defaultOpts,

			inDockerfilePath: "world/hello/Dockerfile",
			wantedError:      errors.New("open world/hello/Dockerfile: file does not exist"),
		},
		"malformed image name": {
			basicOpts: defaultOpts,

			inImage: "image name",
			wantedError: errors.New("image name is malformed"),
		},
		"malformed subnet id": {
			basicOpts: defaultOpts,

			inSubnetID: "malformed-subnet-7192f73d",
			wantedError: errors.New("subnet id is malformed"),
		},
		"malformed security group ids": {
			basicOpts: defaultOpts,

			inSecurityGroupIDs: []string{"malformed-sg-12d9dj", "sg-123dfda9"},
			wantedError: errors.New("one or more malformed security group id(s)"),
		},
		"specified app exists": {
			basicOpts: defaultOpts,

			inApp: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},

			wantedError: nil,
		},
		"unknown app": {
			basicOpts: defaultOpts,

			inApp: "my-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "my-app",
					AccountID: "115",
					Region: "us-east-1",
				})
			},
			wantedError: fmt.Errorf("get application: couldn't find an application named my-app in account 115 and region us-east-1"),
		},
		"env exists in app": {
			basicOpts: defaultOpts,

			inApp: "my-app",
			inEnv: "dev",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "dev").Return(&config.Environment{
					App: "my-app",
					Name: "dev",
				}, nil)

				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: nil,
		},
		"unknown env in specified app": {
			basicOpts: defaultOpts,

			inApp: "my-app",
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
		"env exists in workspace": {
			basicOpts: defaultOpts,

			inEnv: "test",
			appName: "their-app",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("their-app", "test").Return(&config.Environment{
					App: "their-app",
					Name: "test",
				}, nil)
			},
			wantedError: nil,
		},
		"unknown env in workspace": {
			basicOpts: defaultOpts,

			inEnv: "test",
			appName: "their-app",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("their-app", "test").Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: "my-app",
					EnvironmentName: "test",
				})
			},
			wantedError: fmt.Errorf("get environment test: couldn't find environment test in the application my-app"),
		},
		"no workspace": {
			basicOpts: defaultOpts,

			inEnv: "test",
			wantedError: errNoAppInWorkspace,
		},
		"both environment and subnet id specified": {
			basicOpts: defaultOpts,

			inEnv: "test",
			inSubnetID: "subnet id",

			wantedError: errors.New("can only specify one of a)env and b)subnet id and (or) security groups"),
		},
		"both environment and security groups specified": {
			basicOpts: defaultOpts,

			inEnv: "test",
			inSecurityGroupIDs: []string{"security group id1", "securty group id2"},

			wantedError: errors.New("can only specify one of a)env and b)subnet id and (or) security groups"),
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
					Num: tc.inNum,
					CPU: tc.inCPU,
					Memory: tc.inMemory,
					Image: tc.inImage,
					App: tc.inApp,
					Env: tc.inEnv,
					TaskRole: tc.inTaskRole,
					SubnetID: tc.inSubnetID,
					SecurityGroupIDs: tc.inSecurityGroupIDs,
					DockerfilePath: tc.inDockerfilePath,
					EnvVars: tc.inEnvVars,
					Commands: tc.inCommands,
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
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
