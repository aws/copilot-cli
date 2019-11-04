// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
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
			addEnv := &InitEnvOpts{
				EnvName:    tc.inputEnv,
				prompt:     mockPrompter,
				globalOpts: globalOpts{projectName: tc.inputProject},
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

			wantedErr: fmt.Sprintf("environment name 123env is invalid: %s", errValueBadFormat),
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
				EnvName:    tc.inEnvName,
				globalOpts: globalOpts{projectName: tc.inProjectName},
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
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string

		expectProjectGetter func(m *mocks.MockProjectGetter)
		expectEnvCreator    func(m *mocks.MockEnvironmentCreator)
		expectEnvDeployer   func(m *climocks.MockenvironmentDeployer)
		expectIdentity      func(m *climocks.MockidentityService)
		expectProgress      func(m *climocks.Mockprogress)

		wantedErrorS string
	}{
		"returns project exists error": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(nil, errors.New("some error"))
			},

			wantedErrorS: "some error",
		},
		"returns identity get error": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, errors.New("some identity error"))
			},
			wantedErrorS: "get identity: some identity error",
		},
		"stops if environment stack already exists": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start("Proposing infrastructure changes for the test environment")
				m.EXPECT().Stop("")
			},
			expectEnvDeployer: func(m *climocks.MockenvironmentDeployer) {
				m.EXPECT().DeployEnvironment(&deploy.CreateEnvironmentInput{
					Name:                     "test",
					Project:                  "phonetool",
					PublicLoadBalancer:       true,
					ToolsAccountPrincipalARN: "some arn",
				}).Return(&cloudformation.ErrStackAlreadyExists{})
			},
		},
		"errors if environment change set cannot be accepted": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start("Proposing infrastructure changes for the test environment")
				m.EXPECT().Stop(fmt.Sprintf("%s Failed to accept changes for the test environment", color.ErrorMarker))
			},
			expectEnvDeployer: func(m *climocks.MockenvironmentDeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(errors.New("some deploy error"))
			},
			wantedErrorS: "some deploy error",
		},
		"streams failed events": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start("Proposing infrastructure changes for the test environment")
				m.EXPECT().Start("Creating the infrastructure for the test environment")
				m.EXPECT().Events([]string{
					fmt.Sprintf("%s\t[%s]", vpc, failed),
					fmt.Sprintf("  %s\t", "some reason"),
				})
				m.EXPECT().Stop(fmt.Sprintf("%s Failed to create the infrastructure for the test environment", color.ErrorMarker))
			},
			expectEnvDeployer: func(m *climocks.MockenvironmentDeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				events <- []deploy.ResourceEvent{
					{
						LogicalName:  "VPC",
						Type:         "AWS::EC2::VPC",
						Status:       "CREATE_FAILED",
						StatusReason: "some reason",
					},
				}
				responses <- deploy.CreateEnvironmentResponse{
					Err: errors.New("some stream error"),
				}
				close(events)
				close(responses)
			},
			wantedErrorS: "some stream error",
		},
		"returns error from CreateEnvironment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start("Proposing infrastructure changes for the test environment")
				m.EXPECT().Start("Creating the infrastructure for the test environment")
				m.EXPECT().Stop(fmt.Sprintf("%s Created the infrastructure for the test environment", color.SuccessMarker))
			},
			expectEnvDeployer: func(m *climocks.MockenvironmentDeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &archer.Environment{
						Project: "phonetool",
						Name:    "test",
					},
					Err: nil,
				}
				close(events)
				close(responses)
			},
			expectEnvCreator: func(m *mocks.MockEnvironmentCreator) {
				m.EXPECT().CreateEnvironment(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}).Return(errors.New("some create error"))
			},
			wantedErrorS: "store environment: some create error",
		},
		"success": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start("Proposing infrastructure changes for the test environment")
				m.EXPECT().Start("Creating the infrastructure for the test environment")
				m.EXPECT().Stop(fmt.Sprintf("%s Created the infrastructure for the test environment", color.SuccessMarker))
			},
			expectEnvDeployer: func(m *climocks.MockenvironmentDeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &archer.Environment{
						Project: "phonetool",
						Name:    "test",
					},
					Err: nil,
				}
				close(events)
				close(responses)
			},
			expectEnvCreator: func(m *mocks.MockEnvironmentCreator) {
				m.EXPECT().CreateEnvironment(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		//tc := tc // capture range variable
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectGetter := mocks.NewMockProjectGetter(ctrl)
			mockEnvCreator := mocks.NewMockEnvironmentCreator(ctrl)
			mockEnvDeployer := climocks.NewMockenvironmentDeployer(ctrl)
			mockIdentity := climocks.NewMockidentityService(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)
			if tc.expectProjectGetter != nil {
				tc.expectProjectGetter(mockProjectGetter)
			}
			if tc.expectEnvCreator != nil {
				tc.expectEnvCreator(mockEnvCreator)
			}
			if tc.expectEnvDeployer != nil {
				tc.expectEnvDeployer(mockEnvDeployer)
			}
			if tc.expectIdentity != nil {
				tc.expectIdentity(mockIdentity)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}

			opts := &InitEnvOpts{
				EnvName:       tc.inEnvName,
				projectGetter: mockProjectGetter,
				envCreator:    mockEnvCreator,
				envDeployer:   mockEnvDeployer,
				identity:      mockIdentity,
				prog:          mockProgress,
				globalOpts:    globalOpts{projectName: tc.inProjectName},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
