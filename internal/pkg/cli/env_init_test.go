// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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
			addEnv := &InitEnvOpts{
				EnvName:     tc.inputEnv,
				projectName: tc.inputProject,
				prompt:      mockPrompter,
			}
			tc.setupMocks()

			err := addEnv.Ask()

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
			opts := &InitEnvOpts{
				EnvName:     tc.inEnvName,
				projectName: tc.inProjectName,
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockProjStore := mocks.NewMockProjectStore(ctrl)
	mockDeployer := mocks.NewMockEnvironmentDeployer(ctrl)
	mockSpinner := climocks.NewMockprogress(ctrl)
	mockIdentityService := climocks.NewMockidentityService(ctrl)

	var capturedArgument *archer.Environment

	mockError := fmt.Errorf("error")
	mockARN := "mockARN"
	mockCaller := identity.Caller{
		ARN: mockARN,
	}

	testCases := map[string]struct {
		addEnvOpts  InitEnvOpts
		expectedEnv archer.Environment
		expectedErr error
		mocking     func()
	}{
		"with a succesful call to add env": {
			addEnvOpts: InitEnvOpts{
				envCreator:    mockEnvStore,
				projectGetter: mockProjStore,
				envDeployer:   mockDeployer,
				projectName:   "project",
				EnvName:       "env",
				IsProduction:  true,
				prog:          mockSpinner,
				identity:      mockIdentityService,
			},
			expectedEnv: archer.Environment{
				Name:        "env",
				Project:     "project",
				AccountID:   "1234",
				Region:      "1234",
				RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
				Prod:        true,
			},
			mocking: func() {
				gomock.InOrder(
					mockProjStore.
						EXPECT().
						GetProject(gomock.Any()).
						Return(&archer.Project{}, nil),
					mockIdentityService.EXPECT().Get().Times(1).Return(mockCaller, nil),
					mockSpinner.EXPECT().Start(gomock.Eq("Preparing deployment...")),
					mockDeployer.EXPECT().DeployEnvironment(gomock.Any()),
					mockSpinner.EXPECT().Stop(gomock.Eq("Done!")),
					mockSpinner.EXPECT().Start(gomock.Eq("Deploying env...")),
					// TODO: Assert Wait is called with stack name returned by DeployEnvironment.
					mockDeployer.EXPECT().
						WaitForEnvironmentCreation(gomock.Any()).
						Return(&archer.Environment{
							Name:        "env",
							Project:     "project",
							AccountID:   "1234",
							Region:      "1234",
							RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
							Prod:        true,
						}, nil),
					mockEnvStore.
						EXPECT().
						CreateEnvironment(gomock.Any()).
						Do(func(env *archer.Environment) {
							capturedArgument = env
						}),
					mockSpinner.EXPECT().Stop(gomock.Eq("Done!")),
				)
			},
		},
		"with an invalid project": {
			expectedErr: errors.New("retrieve project project: error"),
			addEnvOpts: InitEnvOpts{
				envCreator:    mockEnvStore,
				projectGetter: mockProjStore,
				envDeployer:   mockDeployer,
				projectName:   "project",
				EnvName:       "env",
				IsProduction:  true,
				prog:          mockSpinner,
			},
			expectedEnv: archer.Environment{
				Name:        "env",
				Project:     "project",
				AccountID:   "1234",
				Region:      "1234",
				RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
				Prod:        true,
			},
			mocking: func() {
				mockProjStore.
					EXPECT().
					GetProject(gomock.Any()).
					Return(nil, mockError)
				mockEnvStore.
					EXPECT().
					CreateEnvironment(gomock.Any()).
					Times(0)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup mocks
			tc.mocking()

			err := tc.addEnvOpts.Execute()
			if tc.expectedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedEnv, *capturedArgument)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}
