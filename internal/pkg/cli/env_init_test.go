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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
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
				GlobalOpts: &GlobalOpts{projectName: tc.inputProject},
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

			wantedErr: "no project found, run `project init` first please",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &InitEnvOpts{
				EnvName:    tc.inEnvName,
				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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
		expectDeployer      func(m *climocks.Mockdeployer)
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
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop("")
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
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
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop(log.Serrorf(fmtDeployEnvFailed, "test"))
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
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
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Events([]termprogress.TabRow{
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textVPC, termprogress.StatusFailed)),
					termprogress.TabRow(fmt.Sprintf("  %s\t", "some reason")),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textInternetGateway, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPublicSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPrivateSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textNATGateway, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textRouteTables, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textECSCluster, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textALB, termprogress.StatusInProgress)),
				})
				m.EXPECT().Stop(log.Serrorf(fmtStreamEnvFailed, "test"))
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				events <- []deploy.ResourceEvent{
					{
						Resource: deploy.Resource{
							LogicalName: "VPC",
							Type:        "AWS::EC2::VPC",
						},
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
		"failed to create stack set instance": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectProjectGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *climocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{ARN: "some arn"}, nil)
			},
			expectProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Serrorf(fmtAddEnvToProjectFailed, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				env := &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}
				responses <- deploy.CreateEnvironmentResponse{
					Env: env,
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().AddEnvToProject(&archer.Project{Name: "phonetool"}, env).Return(errors.New("some cfn error"))
			},
			expectEnvCreator: func(m *mocks.MockEnvironmentCreator) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
			},
			wantedErrorS: "deploy env test to project phonetool: some cfn error",
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
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().AddEnvToProject(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectEnvCreator: func(m *mocks.MockEnvironmentCreator) {
				m.EXPECT().CreateEnvironment(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
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
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *climocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().AddEnvToProject(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectEnvCreator: func(m *mocks.MockEnvironmentCreator) {
				m.EXPECT().CreateEnvironment(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectGetter := mocks.NewMockProjectGetter(ctrl)
			mockEnvCreator := mocks.NewMockEnvironmentCreator(ctrl)
			mockDeployer := climocks.NewMockdeployer(ctrl)
			mockIdentity := climocks.NewMockidentityService(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)
			if tc.expectProjectGetter != nil {
				tc.expectProjectGetter(mockProjectGetter)
			}
			if tc.expectEnvCreator != nil {
				tc.expectEnvCreator(mockEnvCreator)
			}
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
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
				envDeployer:   mockDeployer,
				projDeployer:  mockDeployer,
				identity:      mockIdentity,
				prog:          mockProgress,
				GlobalOpts:    &GlobalOpts{projectName: tc.inProjectName},
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
