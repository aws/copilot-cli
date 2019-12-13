// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inEnvName    string
		inEnvProfile string

		mockPrompt func(ctrl *gomock.Controller) *climocks.Mockprompter

		wantedEnvName    string
		wantedEnvProfile string
		wantedError      error
	}{
		"prompts for all required flags": {
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				p := climocks.NewMockprompter(ctrl)
				p.EXPECT().Get(envDeleteNamePrompt, envDeleteNameHelpPrompt, gomock.Any()).Return("test", nil)
				p.EXPECT().Get(fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput("test")),
					envDeleteProfileHelpPrompt, gomock.Any(), gomock.Any()).Return("default", nil)
				return p
			},
			wantedEnvName:    "test",
			wantedEnvProfile: "default",
		},
		"wraps error from prompting for env name": {
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				p := climocks.NewMockprompter(ctrl)
				p.EXPECT().Get(envDeleteNamePrompt, envDeleteNameHelpPrompt, gomock.Any()).Return("", errors.New("some error"))
				return p
			},
			wantedError: errors.New("prompt to get environment name: some error"),
		},
		"wraps error from prompting from profile": {
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				p := climocks.NewMockprompter(ctrl)
				p.EXPECT().Get(envDeleteNamePrompt, envDeleteNameHelpPrompt, gomock.Any()).Return("test", nil)
				p.EXPECT().Get(fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput("test")),
					envDeleteProfileHelpPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
				return p
			},
			wantedError: errors.New("prompt to get the profile name: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := DeleteEnvOpts{
				EnvName:    tc.inEnvName,
				EnvProfile: tc.inEnvProfile,
				GlobalOpts: &GlobalOpts{
					prompt: tc.mockPrompt(ctrl),
				},
			}

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

func TestDeleteEnvOpts_Validate(t *testing.T) {
	const (
		testProjName       = "phonetool"
		testEnvName        = "test"
		testRegion         = "us-west-2"
		testManagerRoleARN = "arn:aws:iam::1111:role/phonetool-test-EnvManagerRole"
	)
	var storeWithEnv = func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
		envStore := mocks.NewMockEnvironmentStore(ctrl)
		envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(&archer.Environment{
			Project:        testProjName,
			Name:           testEnvName,
			Region:         testRegion,
			ManagerRoleARN: testManagerRoleARN,
		}, nil)
		return envStore
	}

	testCases := map[string]struct {
		inProjectName string
		inEnv         string
		mockStore     func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore
		mockRG        func(ctrl *gomock.Controller) *climocks.MockresourceGetter

		wantedError error
	}{
		"failed to retrieve environment from store": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				envStore := mocks.NewMockEnvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testProjName, testEnvName).Return(nil, errors.New("some error"))
				return envStore
			},
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				return nil
			},
			wantedError: errors.New("get environment test metadata in project phonetool: some error"),
		},
		"failed to get resources with tags": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))
				return rg
			},
			wantedError: errors.New("find application cloudformation stacks: some error"),
		},
		"environment has applications": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{
						{
							Tags: []*resourcegroupstaggingapi.Tag{
								{
									Key:   aws.String(stack.AppTagKey),
									Value: aws.String("frontend"),
								},
								{
									Key:   aws.String(stack.AppTagKey),
									Value: aws.String("backend"),
								},
							},
						},
					},
				}, nil)
				return rg
			},
			wantedError: errors.New("applications: 'frontend, backend' still exist within the environment test"),
		},
		"success on empty environment": {
			inProjectName: testProjName,
			inEnv:         testEnvName,
			mockStore:     storeWithEnv,
			mockRG: func(ctrl *gomock.Controller) *climocks.MockresourceGetter {
				rg := climocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)
				return rg
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &DeleteEnvOpts{
				EnvName:     tc.inEnv,
				storeClient: tc.mockStore(ctrl),
				rgClient:    tc.mockRG(ctrl),
				GlobalOpts:  &GlobalOpts{projectName: tc.inProjectName},
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

func TestDeleteEnvOpts_Execute(t *testing.T) {
	const (
		testProject = "phonetool"
		testEnv     = "test"
	)
	testError := errors.New("some error")

	testCases := map[string]struct {
		inSkipPrompt bool
		mockPrompt   func(ctrl *gomock.Controller) *climocks.Mockprompter
		mockProg     func(ctrl *gomock.Controller) *climocks.Mockprogress
		mockDeploy   func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer
		mockStore    func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore

		wantedError error
	}{
		"error from prompt": {
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				prompt := climocks.NewMockprompter(ctrl)
				prompt.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv, testProject), gomock.Any()).Return(false, testError)
				return prompt
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				return climocks.NewMockprogress(ctrl)
			},
			mockDeploy: func(ctrl *gomock.Controller) *climocks.MockenvironmentDeployer {
				return climocks.NewMockenvironmentDeployer(ctrl)
			},
			mockStore: func(ctrl *gomock.Controller) *mocks.MockEnvironmentStore {
				return mocks.NewMockEnvironmentStore(ctrl)
			},

			wantedError: errors.New("prompt for environment deletion: some error"),
		},
		"error from delete stack": {
			inSkipPrompt: true,
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				return climocks.NewMockprompter(ctrl)
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				prog := climocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testProject))
				prog.EXPECT().Stop(fmt.Sprintf(fmtDeleteEnvFailed, testEnv, testProject, testError))
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
			inSkipPrompt: true,
			mockPrompt: func(ctrl *gomock.Controller) *climocks.Mockprompter {
				return climocks.NewMockprompter(ctrl)
			},
			mockProg: func(ctrl *gomock.Controller) *climocks.Mockprogress {
				prog := climocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(fmt.Sprintf(fmtDeleteEnvStart, testEnv, testProject))
				prog.EXPECT().Stop(fmt.Sprintf(fmtDeleteEnvComplete, testEnv, testProject))
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
			opts := DeleteEnvOpts{
				EnvName:          testEnv,
				SkipConfirmation: tc.inSkipPrompt,
				storeClient:      tc.mockStore(ctrl),
				deployClient:     tc.mockDeploy(ctrl),
				prog:             tc.mockProg(ctrl),
				env: &archer.Environment{
					Project: testProject,
					Name:    testEnv,
				},
				GlobalOpts: &GlobalOpts{
					projectName: testProject,
					prompt:      tc.mockPrompt(ctrl),
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
