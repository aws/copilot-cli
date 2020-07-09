// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testAppName        = "badgoose"
	testPipelineName   = "honkpipes"
	testPipelineSecret = "honkhonkhonk"
)

type deletePipelineMocks struct {
	prompt         *mocks.Mockprompter
	prog           *mocks.Mockprogress
	secretsmanager *mocks.MocksecretsManager
	deployer       *mocks.MockpipelineDeployer
	ws             *mocks.MockwsPipelineReader
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
		inAppName string
		callMocks func(m deletePipelineMocks)

		wantedError error
	}{
		"happy path": {
			inAppName: testAppName,
			callMocks: func(m deletePipelineMocks) {
				m.ws.EXPECT().ReadPipelineManifest().Return([]byte(pipelineData), nil)
			},
			wantedError: nil,
		},

		"pipeline manifest does not exist": {
			inAppName: testAppName,
			callMocks: func(m deletePipelineMocks) {
				m.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace)
			},

			wantedError: workspace.ErrNoPipelineInWorkspace,
		},

		"application does not exist": {
			inAppName: "",
			callMocks: func(m deletePipelineMocks) {},

			wantedError: errNoAppInWorkspace,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mocks := deletePipelineMocks{
				ws: mockWorkspace,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
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
		inAppName        string
		inPipelineName   string

		callMocks func(m deletePipelineMocks)

		wantedError error
	}{
		"skips confirmation works": {
			skipConfirmation: true,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,

			callMocks: func(m deletePipelineMocks) {},

			wantedError: nil,
		},

		"delete confirmation works": {
			skipConfirmation: false,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(pipelineDeleteConfirmPrompt, testPipelineName, testAppName),
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

			mockPrompt := mocks.NewMockprompter(ctrl)

			mocks := deletePipelineMocks{
				prompt: mockPrompt,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					SkipConfirmation: tc.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
						prompt:  mockPrompt,
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
	testCases := map[string]struct {
		deleteSecret     bool
		inAppName        string
		inPipelineName   string
		inPipelineSecret string

		setupMocks func(mocks deletePipelineMocks)

		wantedError error
	}{
		"skips delete secret confirmation when flag is specified": {
			deleteSecret:     true,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					// no confirmation prompt for deleting secret
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(testPipelineName).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"asks for confirmation when delete secret flag is not specified": {
			deleteSecret:     false,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.prompt.EXPECT().Confirm(
						fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, testPipelineSecret, testPipelineName),
						pipelineDeleteSecretConfirmHelp,
					).Times(1).Return(true, nil),
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(testPipelineName).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"does not delete secret if user does not confirm": {
			deleteSecret:     false,
			inAppName:        testAppName,
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
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(testPipelineName).Times(1).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"error when deleting stack": {
			deleteSecret:     true,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			inPipelineSecret: testPipelineSecret,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(testPipelineName).Times(1).Return(testError),
					mocks.prog.EXPECT().Stop(log.Serrorf(fmtDeletePipelineFailed, testPipelineName, testAppName, testError)),
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

			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockProg := mocks.NewMockprogress(ctrl)
			mockDeployer := mocks.NewMockpipelineDeployer(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)

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
						appName: tc.inAppName,
						prompt:  mockPrompter,
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
