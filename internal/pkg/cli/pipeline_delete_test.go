// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	awsmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks" // TODO refactor
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testProjName       = "badgoose"
	testPipelineName   = "honkpipes"
	testPipelineSecret = "honkhonkhonk"
)

type deletePipelineMocks struct {
	prompt         *climocks.Mockprompter
	prog           *climocks.Mockprogress
	secretsmanager *awsmocks.MockSecretsManager
	deployer       *climocks.MockpipelineDeployer
	ws             *climocks.MockwsPipelineDeleter
}

func TestDeletePipelineOpts_Validate(t *testing.T) {
	pipelineData := `
name: pipeline-badgoose-honker-repo
version: 1

source:
  provider: GitHub
  properties:
    repository: badgoose/repo
    access_token_secret: "github-token-badgoose-repo"
    branch: master

stages:
    -
      name: test
    -
      name: prod
`

	testCases := map[string]struct {
		inProjectName string
		callMocks     func(m deletePipelineMocks)

		wantedError error
	}{
		"happy path": {
			inProjectName: testProjName,
			callMocks: func(m deletePipelineMocks) {
				m.ws.EXPECT().ReadPipelineManifest().Return([]byte(pipelineData), nil)
			},
			wantedError: nil,
		},

		"pipeline manifest does not exist": {
			inProjectName: testProjName,
			callMocks: func(m deletePipelineMocks) {
				m.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace)
			},

			wantedError: workspace.ErrNoPipelineInWorkspace,
		},

		"project does not exist": {
			inProjectName: "",
			callMocks:     func(m deletePipelineMocks) {},

			wantedError: errNoProjectInWorkspace,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := climocks.NewMockwsPipelineDeleter(ctrl)
			mocks := deletePipelineMocks{
				ws: mockWorkspace,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
					},
				},
				ws: mockWorkspace,
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

func TestDeletePipelineOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		skipConfirmation bool
		inProjectName    string
		inPipelineName   string

		callMocks func(m deletePipelineMocks)

		wantedError error
	}{
		"skips confirmation works": {
			skipConfirmation: true,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,

			callMocks: func(m deletePipelineMocks) {},

			wantedError: nil,
		},

		"delete confirmation works": {
			skipConfirmation: false,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(pipelineDeleteConfirmPrompt, testPipelineName, testProjName),
					pipelineDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},
			wantedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := climocks.NewMockprompter(ctrl)

			mocks := deletePipelineMocks{
				prompt: mockPrompt,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					SkipConfirmation: tc.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
						prompt:      mockPrompt,
					},
				},
				PipelineName: tc.inPipelineName,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeletePipelineOpts_Execute(t *testing.T) {
	testError := errors.New("some error")
	stackName := testProjName + "-" + testPipelineName

	testCases := map[string]struct {
		deleteSecret     bool
		inProjectName    string
		inPipelineName   string
		inPipelineSecret string

		setupMocks func(mocks deletePipelineMocks)

		wantedError error
	}{
		"skips delete secret confirmation when flag is specified": {
			deleteSecret:     true,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					// no confirmation prompt for deleting secret
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testProjName)),
					mocks.deployer.EXPECT().DeletePipeline(stackName).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testProjName)),
					mocks.ws.EXPECT().DeletePipelineManifest().Return(nil),
				)
			},
			wantedError: nil,
		},

		"asks for confirmation when delete secret flag is not specified": {
			deleteSecret:     false,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.prompt.EXPECT().Confirm(
						fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, testPipelineSecret, testPipelineName),
						pipelineDeleteSecretConfirmHelp,
					).Times(1).Return(true, nil),
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testProjName)),
					mocks.deployer.EXPECT().DeletePipeline(stackName).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testProjName)),
					mocks.ws.EXPECT().DeletePipelineManifest().Return(nil),
				)
			},
			wantedError: nil,
		},

		"does not delete secret if user does not confirm": {
			deleteSecret:     false,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.prompt.EXPECT().Confirm(
						fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, testPipelineSecret, testPipelineName),
						pipelineDeleteSecretConfirmHelp,
					).Times(1).Return(false, nil),

					// does not delete secret
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Times(0),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testProjName)),
					mocks.deployer.EXPECT().DeletePipeline(stackName).Times(1).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testProjName)),
					mocks.ws.EXPECT().DeletePipelineManifest().Return(nil),
				)
			},
			wantedError: nil,
		},

		"error when deleting stack": {
			deleteSecret:     true,
			inProjectName:    testProjName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testProjName)),
					mocks.deployer.EXPECT().DeletePipeline(stackName).Times(1).Return(testError),
					mocks.prog.EXPECT().Stop(log.Serrorf(fmtDeletePipelineFailed, testPipelineName, testProjName, testError)),
					mocks.ws.EXPECT().DeletePipelineManifest().Times(0),
				)
			},
			wantedError: testError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := awsmocks.NewMockSecretsManager(ctrl)
			mockProg := climocks.NewMockprogress(ctrl)
			mockDeployer := climocks.NewMockpipelineDeployer(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)
			mockWorkspace := climocks.NewMockwsPipelineDeleter(ctrl)

			mocks := deletePipelineMocks{
				prompt:         mockPrompter,
				prog:           mockProg,
				secretsmanager: mockSecretsManager,
				deployer:       mockDeployer,
				ws:             mockWorkspace,
			}

			tc.setupMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					DeleteSecret: tc.deleteSecret,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
						prompt:      mockPrompter,
					},
				},
				PipelineName:     tc.inPipelineName,
				PipelineSecret:   tc.inPipelineSecret,
				secretsmanager:   mockSecretsManager,
				pipelineDeployer: mockDeployer,
				ws:               mockWorkspace,
				prog:             mockProg,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}
