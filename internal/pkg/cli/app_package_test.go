// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestPackageAppOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string

		expectAppStore func(m *mocks.MockApplicationStore)
		expectEnvStore func(m *mocks.MockEnvironmentStore)
		expectPrompt   func(m *climocks.Mockprompter)

		wantedAppName string
		wantedEnvName string
		wantedErrorS  string
	}{
		"wrap list apps error": {
			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Return(nil, errors.New("some ssm error"))
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications for project : some ssm error",
		},
		"wrap list envs error": {
			inAppName: "frontend",
			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, errors.New("some ssm error"))
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedAppName: "frontend",
			wantedErrorS:  "list environments for project : some ssm error",
		},
		"prompt for all options": {
			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Return([]*archer.Application{
					{
						Name: "frontend",
					},
					{
						Name: "backend",
					},
				}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne("Which application's CloudFormation template would you like to generate?", gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
				m.EXPECT().SelectOne("Which environment's configuration would you like to use for your stack's parameters?", gomock.Any(), []string{"test", "prod"}).Return("test", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"prompt only for the app name": {
			inEnvName: "test",

			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Return([]*archer.Application{
					{
						Name: "frontend",
					},
					{
						Name: "backend",
					},
				}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne("Which application's CloudFormation template would you like to generate?", gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"prompt only for the env name": {
			inAppName: "frontend",

			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne("Which environment's configuration would you like to use for your stack's parameters?", gomock.Any(), []string{"test", "prod"}).Return("test", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"don't prompt": {
			inAppName: "frontend",
			inEnvName: "test",

			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().ListApplications(gomock.Any()).Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAppStore := mocks.NewMockApplicationStore(ctrl)
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)

			tc.expectAppStore(mockAppStore)
			tc.expectEnvStore(mockEnvStore)
			tc.expectPrompt(mockPrompt)

			opts := &PackageAppOpts{
				AppName:  tc.inAppName,
				EnvName:  tc.inEnvName,
				appStore: mockAppStore,
				envStore: mockEnvStore,
				prompt:   mockPrompt,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedAppName, opts.AppName)
			require.Equal(t, tc.wantedEnvName, opts.EnvName)

			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestPackageAppOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inAppName     string

		expectAppStore func(m *mocks.MockApplicationStore)
		expectEnvStore func(m *mocks.MockEnvironmentStore)

		wantedErrorS string
	}{
		"invalid workspace": {
			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication(gomock.Any(), gomock.Any()).Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "could not find a project attached to this workspace, please run `project init` first",
		},
		"error while fetching application": {
			inProjectName: "phonetool",
			inAppName:     "frontend",

			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("phonetool", "frontend").Return(nil, &store.ErrNoSuchApplication{
					ProjectName:     "phonetool",
					ApplicationName: "frontend",
				})
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: (&store.ErrNoSuchApplication{
				ProjectName:     "phonetool",
				ApplicationName: "frontend",
			}).Error(),
		},
		"error while fetching environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication(gomock.Any(), gomock.Any()).Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &store.ErrNoSuchEnvironment{
					ProjectName:     "phonetool",
					EnvironmentName: "test",
				})
			},

			wantedErrorS: (&store.ErrNoSuchEnvironment{
				ProjectName:     "phonetool",
				EnvironmentName: "test",
			}).Error(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAppStore := mocks.NewMockApplicationStore(ctrl)
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			tc.expectAppStore(mockAppStore)
			tc.expectEnvStore(mockEnvStore)

			viper.Set(projectFlag, tc.inProjectName)
			defer viper.Set(projectFlag, "")
			opts := &PackageAppOpts{
				AppName:  tc.inAppName,
				EnvName:  tc.inEnvName,
				appStore: mockAppStore,
				envStore: mockEnvStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS, "error %v does not match '%s'", err, tc.wantedErrorS)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestPackageAppOpts_Execute(t *testing.T) {

}
