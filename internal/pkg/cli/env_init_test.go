// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
)

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

			wantedErr: "no project found: run `project init` or `cd` into your workspace please",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					EnvName:    tc.inEnvName,
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
				},
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

func TestInitEnvOpts_Ask(t *testing.T) {

	mockEnv := "test"
	mockProfile := "default"

	testCases := map[string]struct {
		inputEnv     string
		inputProfile string
		inputProject string

		setupMocks func(*mocks.Mockprompter, *mocks.MockprofileNames)

		wantedError error
	}{
		"with no flags set": {
			setupMocks: func(mockPrompter *mocks.Mockprompter, mockCfg *mocks.MockprofileNames) {
				mockPrompter.EXPECT().
					Get(
						gomock.Eq(envInitNamePrompt),
						gomock.Eq(envInitNameHelpPrompt),
						gomock.Any()).
					Return(mockEnv, nil)
				mockCfg.EXPECT().Names().Return([]string{mockProfile})
				mockPrompter.EXPECT().
					SelectOne(
						gomock.Eq(fmt.Sprintf(fmtEnvInitProfilePrompt, mockEnv)),
						gomock.Eq(envInitProfileHelpPrompt),
						gomock.Any()).
					Return(mockProfile, nil)
			},
		},
		"with no existing named profiles": {
			setupMocks: func(mockPrompter *mocks.Mockprompter, mockCfg *mocks.MockprofileNames) {
				mockPrompter.EXPECT().
					Get(
						gomock.Eq(envInitNamePrompt),
						gomock.Eq(envInitNameHelpPrompt),
						gomock.Any()).
					Return(mockEnv, nil)
				mockCfg.EXPECT().Names().Return([]string{})
			},
			wantedError: errNamedProfilesNotFound,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mocks.NewMockprompter(ctrl)
			mockCfg := mocks.NewMockprofileNames(ctrl)
			// GIVEN
			addEnv := &initEnvOpts{
				initEnvVars: initEnvVars{
					EnvName:    tc.inputEnv,
					EnvProfile: tc.inputProfile,
					GlobalOpts: &GlobalOpts{
						prompt:      mockPrompter,
						projectName: tc.inputProject,
					},
				},
				profileConfig: mockCfg,
			}
			tc.setupMocks(mockPrompter, mockCfg)

			// WHEN
			err := addEnv.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, mockEnv, addEnv.EnvName, "expected environment names to match")
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestInitEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inProd        bool

		expectstore    func(m *mocks.Mockstore)
		expectDeployer func(m *mocks.Mockdeployer)
		expectIdentity func(m *mocks.MockidentityService)
		expectProgress func(m *mocks.Mockprogress)

		wantedErrorS string
	}{
		"returns project exists error": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},

			wantedErrorS: "some error",
		},
		"returns identity get error": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, errors.New("some identity error"))
			},
			wantedErrorS: "get identity: some identity error",
		},
		"errors if environment change set cannot be accepted": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop(log.Serrorf(fmtDeployEnvFailed, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(errors.New("some deploy error"))
			},
			wantedErrorS: "some deploy error",
		},
		"streams failed events": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Events([]termprogress.TabRow{
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textVPC, termprogress.StatusFailed)),
					termprogress.TabRow(fmt.Sprintf("  %s\t", "some reason")),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textInternetGateway, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPublicSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPrivateSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textRouteTables, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textECSCluster, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textALB, termprogress.StatusInProgress)),
				})
				m.EXPECT().Stop(log.Serrorf(fmtStreamEnvFailed, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
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
		"failed to get environment stack": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				env := &config.Environment{
					App:       "phonetool",
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
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("some error"))
			},
			wantedErrorS: "get environment struct for test: some error",
		},
		"failed to create stack set instance": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Serrorf(fmtAddEnvToProjectFailed, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				env := &config.Environment{
					App:       "phonetool",
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
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(&config.Application{Name: "phonetool"}, env).Return(errors.New("some cfn error"))
			},
			wantedErrorS: "deploy env test to project phonetool: some cfn error",
		},
		"returns error from CreateEnvironment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name: "phonetool",
				}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(errors.New("some create error"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantedErrorS: "store environment: some create error",
		},
		"success": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inProd:        true,

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Prod:      true,
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Prod:      true,
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					Prod:      false,
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"skips creating stack if environment stack already exists": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop("")
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(&deploy.CreateEnvironmentInput{
					Name:                     "test",
					AppName:                  "phonetool",
					PublicLoadBalancer:       true,
					ToolsAccountPrincipalARN: "some arn",
				}).Return(&cloudformation.ErrStackAlreadyExists{})
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"failed to delegate DNS (project has Domain and env and project are different)": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(1)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Serrorf(fmtDNSDelegationFailed, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(errors.New("some error"))
			},
			wantedErrorS: "granting DNS permissions: some error",
		},
		"success with DNS Delegation (project has Domain and env and project are different)": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(2)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToProjectStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockIdentity := mocks.NewMockidentityService(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			if tc.expectstore != nil {
				tc.expectstore(mockstore)
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

			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					EnvName:      tc.inEnvName,
					GlobalOpts:   &GlobalOpts{projectName: tc.inProjectName},
					IsProduction: tc.inProd,
				},
				store:        mockstore,
				envDeployer:  mockDeployer,
				projDeployer: mockDeployer,
				identity:     mockIdentity,
				envIdentity:  mockIdentity,
				prog:         mockProgress,
				initProfileClients: func(o *initEnvOpts) error {
					return nil
				},
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

func TestInitEnvOpts_delegateDNSFromProject(t *testing.T) {
	testCases := map[string]struct {
		project        *config.Application
		expectDeployer func(m *mocks.Mockdeployer)
		expectIdentity func(m *mocks.MockidentityService)
		expectProgress func(m *mocks.Mockprogress)
		wantedErr      string
	}{
		"should call DelegateDNSPermissions when project and env are in different accounts": {
			project: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountproject",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "4567"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
			},
		},
		"should skip updating when project and env are in same account": {
			project: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountproject",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"should return errors from identity": {
			project: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountproject",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, fmt.Errorf("error"))
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "getting environment account ID for DNS Delegation: error",
		},
		"should return errors from DelegateDNSPermissions": {
			project: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountproject",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "4567"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Serrorf(fmtDNSDelegationFailed, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			},
			wantedErr: "error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockIdentity := mocks.NewMockidentityService(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
			}
			if tc.expectIdentity != nil {
				tc.expectIdentity(mockIdentity)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					GlobalOpts: &GlobalOpts{projectName: tc.project.Name},
				},
				envIdentity:  mockIdentity,
				projDeployer: mockDeployer,
				prog:         mockProgress,
			}

			// WHEN
			err := opts.delegateDNSFromProject(tc.project)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
