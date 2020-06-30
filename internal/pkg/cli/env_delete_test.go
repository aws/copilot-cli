// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteEnvOpts_Validate(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
	)
	testCases := map[string]struct {
		inAppName string
		inEnv     string
		mockStore func(ctrl *gomock.Controller) *mocks.MockenvironmentStore

		wantedError error
	}{
		"failed to retrieve environment from store": {
			inAppName: testAppName,
			inEnv:     testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				envStore := mocks.NewMockenvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testAppName, testEnvName).Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: testAppName,
					EnvironmentName: testEnvName,
				})
				return envStore
			},
			wantedError: &config.ErrNoSuchEnvironment{
				ApplicationName: testAppName,
				EnvironmentName: testEnvName,
			},
		},
		"environment exists": {
			inAppName: testAppName,
			inEnv:     testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				envStore := mocks.NewMockenvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testAppName, testEnvName).Return(&config.Environment{}, nil)
				return envStore
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &deleteEnvOpts{
				deleteEnvVars: deleteEnvVars{
					EnvName:    tc.inEnv,
					GlobalOpts: &GlobalOpts{appName: tc.inAppName},
				},
				store: tc.mockStore(ctrl),
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeleteEnvOpts_Ask(t *testing.T) {
	const (
		testApp      = "phonetool"
		testEnv      = "test"
		testProfile1 = "default1"
		testProfile2 = "default2"
	)
	testCases := map[string]struct {
		inEnvName          string
		inEnvProfile       string
		inSkipConfirmation bool

		mockDependencies func(ctrl *gomock.Controller, o *deleteEnvOpts)

		wantedEnvName    string
		wantedEnvProfile string
		wantedError      error
	}{
		"prompts for all required flags": {
			inSkipConfirmation: false,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockSelector := mocks.NewMockconfigSelector(ctrl)
				mockSelector.EXPECT().Environment(envDeleteNamePrompt, "", testApp).Return(testEnv, nil)

				mockCfg := mocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1, testProfile2})

				mockPrompter := mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().SelectOne(fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput(testEnv)),
					envDeleteProfileHelpPrompt, []string{testProfile1, testProfile2}).Return(testProfile1, nil)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv, testApp), gomock.Any()).Return(true, nil)

				o.sel = mockSelector
				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},
			wantedEnvName:    testEnv,
			wantedEnvProfile: testProfile1,
		},
		"skip prompting if only one profile available": {
			inSkipConfirmation: true,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockSelector := mocks.NewMockconfigSelector(ctrl)
				mockSelector.EXPECT().Environment(envDeleteNamePrompt, "", testApp).Return(testEnv, nil)

				mockCfg := mocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1})

				mockPrompter := mocks.NewMockprompter(ctrl)

				o.sel = mockSelector
				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},
			wantedEnvName:    testEnv,
			wantedEnvProfile: testProfile1,
		},
		"wraps error from prompting for confirmation": {
			inSkipConfirmation: false,
			inEnvName:          testEnv,
			inEnvProfile:       testProfile1,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {

				mockPrompter := mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv, testApp), gomock.Any()).Return(false, errors.New("some error"))

				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("confirm to delete environment test: some error"),
		},
		"wraps error from prompting from profile": {
			inSkipConfirmation: true,
			inEnvName:          testEnv,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockCfg := mocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1, testProfile2})

				mockPrompter := mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))

				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("get the profile name: some error"),
		},
		"errors when no named profile exists": {
			inSkipConfirmation: true,
			inEnvName:          testEnv,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockCfg := mocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{})

				o.profileConfig = mockCfg
			},

			wantedError: errNamedProfilesNotFound,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &deleteEnvOpts{
				deleteEnvVars: deleteEnvVars{
					EnvName:    tc.inEnvName,
					EnvProfile: tc.inEnvProfile,
					GlobalOpts: &GlobalOpts{
						appName: testApp,
					},
					SkipConfirmation: tc.inSkipConfirmation,
				},
			}
			tc.mockDependencies(ctrl, opts)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.Equal(t, tc.wantedEnvName, opts.EnvName)
				require.Equal(t, tc.wantedEnvProfile, opts.EnvProfile)
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeleteEnvOpts_Execute(t *testing.T) {
	const (
		testApp = "phonetool"
		testEnv = "test"
	)
	testError := errors.New("some error")

	testCases := map[string]struct {
		mockRG     func(ctrl *gomock.Controller) *mocks.MockresourceGetter
		mockProg   func(ctrl *gomock.Controller) *mocks.Mockprogress
		mockDeploy func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer
		mockStore  func(ctrl *gomock.Controller) *mocks.MockenvironmentStore

		wantedError error
	}{
		"failed to get resources with tags": {
			mockRG: func(ctrl *gomock.Controller) *mocks.MockresourceGetter {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *mocks.Mockprogress {
				return nil
			},
			mockDeploy: func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer {
				return nil
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				return nil
			},
			wantedError: errors.New("find service cloudformation stacks: some error"),
		},
		"environment has running applications": {
			mockRG: func(ctrl *gomock.Controller) *mocks.MockresourceGetter {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{
						{
							Tags: []*resourcegroupstaggingapi.Tag{
								{
									Key:   aws.String(stack.ServiceTagKey),
									Value: aws.String("frontend"),
								},
								{
									Key:   aws.String(stack.ServiceTagKey),
									Value: aws.String("backend"),
								},
							},
						},
					},
				}, nil)
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *mocks.Mockprogress {
				return nil
			},
			mockDeploy: func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer {
				return nil
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				return nil
			},
			wantedError: errors.New("service 'frontend, backend' still exist within the environment test"),
		},
		"error from delete stack": {
			mockRG: func(ctrl *gomock.Controller) *mocks.MockresourceGetter {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *mocks.Mockprogress {
				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testApp))
				prog.EXPECT().Stop(log.Serrorf(fmtDeleteEnvFailed, testEnv, testApp, testError))
				return prog
			},
			mockDeploy: func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer {
				deploy := mocks.NewMockenvironmentDeployer(ctrl)
				deploy.EXPECT().DeleteEnvironment(testApp, testEnv).Return(testError)
				return deploy
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				return mocks.NewMockenvironmentStore(ctrl)
			},
		},
		"deletes from store if stack deletion succeeds": {
			mockRG: func(ctrl *gomock.Controller) *mocks.MockresourceGetter {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *mocks.Mockprogress {
				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testApp))
				prog.EXPECT().Stop(log.Ssuccessf(fmtDeleteEnvComplete, testEnv, testApp))
				return prog
			},
			mockDeploy: func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer {
				deploy := mocks.NewMockenvironmentDeployer(ctrl)
				deploy.EXPECT().DeleteEnvironment(testApp, testEnv).Return(nil)
				return deploy
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				store := mocks.NewMockenvironmentStore(ctrl)
				store.EXPECT().DeleteEnvironment(testApp, testEnv).Return(nil)
				return store
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := deleteEnvOpts{
				deleteEnvVars: deleteEnvVars{
					EnvName: testEnv,
					GlobalOpts: &GlobalOpts{
						appName: testApp,
					},
				},
				store:        tc.mockStore(ctrl),
				deployClient: tc.mockDeploy(ctrl),
				rgClient:     tc.mockRG(ctrl),
				prog:         tc.mockProg(ctrl),
				initProfileClients: func(o *deleteEnvOpts) error {
					return nil
				},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
