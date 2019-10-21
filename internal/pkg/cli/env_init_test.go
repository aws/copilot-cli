// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvAdd_Ask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := climocks.NewMockprompter(ctrl)

	mockProject := "mockProject"
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
							gomock.Eq("What is your project's name?"),
							gomock.Eq("A project groups all of your environments together."),
							gomock.Any()).
						Return(mockProject, nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What is your environment's name?"),
							gomock.Eq("A unique identifier for an environment (e.g. dev, test, prod)"),
							gomock.Any()).
						Return(mockEnv, nil).
						Times(1))
			},
		},
		"with env flags set": {
			inputEnv: mockEnv,
			setupMocks: func() {
				mockPrompter.EXPECT().
					Get(
						gomock.Eq("What is your project's name?"),
						gomock.Eq("A project groups all of your environments together."),
						gomock.Any()).
					Return(mockProject, nil).
					Times(1)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			addEnv := &InitEnvOpts{
				EnvName:     tc.inputEnv,
				ProjectName: tc.inputProject,
				prompter:    mockPrompter,
			}
			tc.setupMocks()

			err := addEnv.Ask()

			require.NoError(t, err)
			require.Equal(t, mockProject, addEnv.ProjectName, "expected project names to match")
			require.Equal(t, mockEnv, addEnv.EnvName, "expected environment names to match")
		})
	}
}

func TestEnvAdd_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockProjStore := mocks.NewMockProjectStore(ctrl)
	mockDeployer := mocks.NewMockEnvironmentDeployer(ctrl)
	mockSpinner := climocks.NewMockprogress(ctrl)
	var capturedArgument *archer.Environment
	defer ctrl.Finish()

	testCases := map[string]struct {
		addEnvOpts  InitEnvOpts
		expectedEnv archer.Environment
		expectedErr error
		mocking     func()
	}{
		"with a succesful call to add env": {
			addEnvOpts: InitEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjStore,
				deployer:      mockDeployer,
				ProjectName:   "project",
				EnvName:       "env",
				Production:    true,
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
				gomock.InOrder(
					mockProjStore.
						EXPECT().
						GetProject(gomock.Any()).
						Return(&archer.Project{}, nil),
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
		"with a invalid project": {
			expectedErr: mockError,
			addEnvOpts: InitEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjStore,
				deployer:      mockDeployer,
				ProjectName:   "project",
				EnvName:       "env",
				Production:    true,
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
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}
