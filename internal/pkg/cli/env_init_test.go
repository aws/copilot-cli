// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
)

func TestInitEnvOpts_Ask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := climocks.NewMockprompter(ctrl)

	mockEnv := "mockEnv"

	testCases := map[string]struct {
		inputEnv     string
		inputProject string

		setupMocks func()
	}{
		"with no flags set": {
			setupMocks: func() {
				gomock.InOrder(
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What is your environment's name?"),
							gomock.Eq("A unique identifier for an environment (e.g. dev, test, prod)"),
							gomock.Any()).
						Return(mockEnv, nil).
						Times(1))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			viper.Set(projectFlag, tc.inputProject)
			addEnv := &InitEnvOpts{
				EnvName: tc.inputEnv,
				prompt:  mockPrompter,
			}
			tc.setupMocks()

			// WHEN
			err := addEnv.Ask()

			// THEN
			require.NoError(t, err)
			require.Equal(t, mockEnv, addEnv.EnvName, "expected environment names to match")
		})
	}
}

func TestInitEnvOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inEnvName     string
		inProjectName string

		wantedErr string
	}{
		"valid environment creation": {
			inEnvName:     "test-pdx",
			inProjectName: "phonetool",
		},
		"invalid environment name": {
			inEnvName:     "123env",
			inProjectName: "phonetool",

			wantedErr: "environment name 123env is invalid: value must be start with letter and container only letters, numbers, and hyphens",
		},
		"new workspace": {
			inEnvName:     "test-pdx",
			inProjectName: "",

			wantedErr: "no project found, run `project init` first",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			viper.Set(projectFlag, tc.inProjectName)
			opts := &InitEnvOpts{
				EnvName: tc.inEnvName,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestInitEnvOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockARN := "mockARN"
	mockCaller := identity.Caller{
		ARN: mockARN,
	}

	testCases := map[string]struct {
		expectedErr error
		mocking     func(t *testing.T, opts *InitEnvOpts)
	}{
		"with a successful call to add env": {
			mocking: func(t *testing.T, opts *InitEnvOpts) {
				gomock.InOrder(
					opts.projectGetter.(*mocks.MockProjectStore).
						EXPECT().
						GetProject(gomock.Any()).
						Return(&archer.Project{}, nil),
					opts.identity.(*climocks.MockidentityService).EXPECT().Get().Times(1).Return(mockCaller, nil),
					opts.prog.(*climocks.Mockprogress).EXPECT().Start(gomock.Eq("Preparing deployment...")),
					opts.envDeployer.(*climocks.MockenvironmentDeployer).EXPECT().DeployEnvironment(gomock.Any()),
					opts.prog.(*climocks.Mockprogress).EXPECT().Stop(gomock.Eq("Done!")),
					opts.prog.(*climocks.Mockprogress).EXPECT().Start(gomock.Eq("Deploying env...")),
					// TODO: Assert Wait is called with stack name returned by DeployEnvironment.
					opts.envDeployer.(*climocks.MockenvironmentDeployer).EXPECT().
						WaitForEnvironmentCreation(gomock.Any()).
						Return(&archer.Environment{
							Name:        "env",
							Project:     "project",
							AccountID:   "1234",
							Region:      "1234",
							RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
							Prod:        true,
						}, nil),
					opts.envCreator.(*mocks.MockEnvironmentStore).
						EXPECT().
						CreateEnvironment(gomock.Any()).
						Do(func(env *archer.Environment) {
							require.Equal(t, &archer.Environment{
								Name:        "env",
								Project:     "project",
								AccountID:   "1234",
								Region:      "1234",
								RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
								Prod:        true,
							}, env)
						}),
					opts.prog.(*climocks.Mockprogress).EXPECT().Stop(gomock.Eq("Done!")),
				)
			},
		},
		"with an existing environment": {
			mocking: func(t *testing.T, opts *InitEnvOpts) {
				opts.projectGetter.(*mocks.MockProjectStore).
					EXPECT().
					GetProject(gomock.Any()).
					Return(&archer.Project{}, nil)
				opts.identity.(*climocks.MockidentityService).EXPECT().Get().Return(mockCaller, nil)
				opts.prog.(*climocks.Mockprogress).EXPECT().Start(gomock.Eq("Preparing deployment..."))
				opts.envDeployer.(*climocks.MockenvironmentDeployer).EXPECT().
					DeployEnvironment(gomock.Any()).
					Return(&cloudformation.ErrStackAlreadyExists{})
				opts.prog.(*climocks.Mockprogress).EXPECT().Stop(gomock.Eq("Done!"))
				opts.envCreator.(*mocks.MockEnvironmentStore).
					EXPECT().
					CreateEnvironment(gomock.Any()).
					Times(0)
			},
		},
		"with an invalid project": {
			expectedErr: mockError,
			mocking: func(t *testing.T, opts *InitEnvOpts) {
				opts.projectGetter.(*mocks.MockProjectStore).
					EXPECT().
					GetProject(gomock.Any()).
					Return(nil, mockError)
				opts.envCreator.(*mocks.MockEnvironmentStore).
					EXPECT().
					CreateEnvironment(gomock.Any()).
					Times(0)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockProjStore := mocks.NewMockProjectStore(ctrl)
			mockDeployer := climocks.NewMockenvironmentDeployer(ctrl)
			mockSpinner := climocks.NewMockprogress(ctrl)
			mockIdentityService := climocks.NewMockidentityService(ctrl)

			viper.Set(projectFlag, "project")
			opts := &InitEnvOpts{
				envCreator:    mockEnvStore,
				projectGetter: mockProjStore,
				envDeployer:   mockDeployer,
				identity:      mockIdentityService,
				EnvName:       "env",
				IsProduction:  true,
				prog:          mockSpinner,
			}
			tc.mocking(t, opts)

			// WHEN
			err := opts.Execute()
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}
