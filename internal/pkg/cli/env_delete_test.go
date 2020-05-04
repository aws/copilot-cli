// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteEnvOpts_Validate(t *testing.T) {
	const (
		testProjName = "phonetool"
		testEnvName  = "test"
	)
	testCases := map[string]struct {
		inProjectName string
		inEnv         string
		mockStore     func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore

		wantedError error
	}{
		"failed to retrieve environment from store": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				envStore := mocks.NewMockEnvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(nil, &store.ErrNoSuchEnvironment{
					ApplicationName: testProjName,
					EnvironmentName: testEnvName,
				})
				return envStore
			},
			wantedError: &store.ErrNoSuchEnvironment{
				ApplicationName: testProjName,
				EnvironmentName: testEnvName,
			},
		},
		"environment exists": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				envStore := mocks.NewMockEnvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(&archer.Environment{}, nil)
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
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
				},
				storeClient: tc.mockStore(ctrl),
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
		testProject  = "phonetool"
		testEnv1     = "test"
		testEnv2     = "prod"
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
				mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
				mockEnvStore.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
					{
						Name: testEnv1,
					},
					{
						Name: testEnv2,
					},
				}, nil)

				mockCfg := climocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1, testProfile2})

				mockPrompter := climocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().SelectOne(envDeleteNamePrompt, "", []string{testEnv1, testEnv2}).Return(testEnv1, nil)
				mockPrompter.EXPECT().SelectOne(fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput(testEnv1)),
					envDeleteProfileHelpPrompt, []string{testProfile1, testProfile2}).Return(testProfile1, nil)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv1, testProject), gomock.Any()).Return(true, nil)

				o.storeClient = mockEnvStore
				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},
			wantedEnvName:    testEnv1,
			wantedEnvProfile: testProfile1,
		},
		"skip prompting if only one environment or profile available": {
			inSkipConfirmation: true,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
				mockEnvStore.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
					{
						Name: testEnv1,
					},
				}, nil)

				mockCfg := climocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1})

				mockPrompter := climocks.NewMockprompter(ctrl)

				o.storeClient = mockEnvStore
				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},
			wantedEnvName:    testEnv1,
			wantedEnvProfile: testProfile1,
		},
		"wraps error from prompting for confirmation": {
			inSkipConfirmation: false,
			inEnvName:          testEnv1,
			inEnvProfile:       testProfile1,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {

				mockPrompter := climocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv1, testProject), gomock.Any()).Return(false, errors.New("some error"))

				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("prompt for environment deletion: some error"),
		},
		"wraps error from prompting for env name": {
			inSkipConfirmation: true,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
				mockEnvStore.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
					{
						Name: testEnv1,
					},
					{
						Name: testEnv2,
					},
				}, nil)

				mockPrompter := climocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().SelectOne(envDeleteNamePrompt, "", gomock.Any()).Return("", errors.New("some error"))

				o.storeClient = mockEnvStore
				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("prompt for environment name: some error"),
		},
		"wraps error if no environment found": {
			inSkipConfirmation: true,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
				mockEnvStore.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{}, nil)

				mockPrompter := climocks.NewMockprompter(ctrl)

				o.storeClient = mockEnvStore
				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("couldn't find any environment in the project phonetool"),
		},
		"wraps error from prompting from profile": {
			inSkipConfirmation: true,
			inEnvName:          testEnv1,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockCfg := climocks.NewMockprofileNames(ctrl)
				mockCfg.EXPECT().Names().Return([]string{testProfile1, testProfile2})

				mockPrompter := climocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))

				o.profileConfig = mockCfg
				o.GlobalOpts.prompt = mockPrompter
			},

			wantedError: errors.New("prompt to get the profile name: some error"),
		},
		"errors when no named profile exists": {
			inSkipConfirmation: true,
			inEnvName:          testEnv1,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockCfg := climocks.NewMockprofileNames(ctrl)
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
						projectName: testProject,
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
		testProject = "phonetool"
		testEnv     = "test"
	)
	testError := errors.New("some error")

	testCases := map[string]struct {
		mockRG     func(ctrl *gomock.Controller) *climocks.MockresourceGetter
		mockProg   func(ctrl *gomock.Controller) *climocks.Mockprogress
		mockDeploy func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer
		mockStore  func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore

		wantedError error
	}{
		"failed to get resources with tags": {
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				return nil
			},
			mockDeploy: func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer {
				return nil
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				return nil
			},
			wantedError: errors.New("find application cloudformation stacks: some error"),
		},
		"environment has running applications": {
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
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
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				return nil
			},
			mockDeploy: func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer {
				return nil
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				return nil
			},
			wantedError: errors.New("applications: 'frontend, backend' still exist within the environment test"),
		},
		"error from delete stack": {
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				prog := climocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testProject))
				prog.EXPECT().Stop(log.Serrorf(fmtDeleteEnvFailed, testEnv, testProject, testError))
				return prog
			},
			mockDeploy: func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer {
				deploy := climocks.NewMockenvironmentDeployer(ctrl)
				deploy.EXPECT().DeleteEnvironment(testProject, testEnv).Return(testError)
				return deploy
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				return mocks.NewMockEnvironmentStore(ctrl)
			},
		},
		"deletes from store if stack deletion succeeds": {
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				prog := climocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testProject))
				prog.EXPECT().Stop(log.Ssuccessf(fmtDeleteEnvComplete, testEnv, testProject))
				return prog
			},
			mockDeploy: func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer {
				deploy := climocks.NewMockenvironmentDeployer(ctrl)
				deploy.EXPECT().DeleteEnvironment(testProject, testEnv).Return(nil)
				return deploy
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				store := mocks.NewMockEnvironmentStore(ctrl)
				store.EXPECT().DeleteEnvironment(testProject, testEnv).Return(nil)
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
						projectName: testProject,
					},
				},
				storeClient:  tc.mockStore(ctrl),
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
