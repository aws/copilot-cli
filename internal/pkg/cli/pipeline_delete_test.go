// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testAppName        = "badgoose"
	testPipelineName   = "honkpipes"
	testPipelineSecret = "honkhonkhonk"

	pipelineManifestLegacyPath = "copilot/pipeline.yml"
)

type deletePipelineMocks struct {
	prompt         *mocks.Mockprompter
	prog           *mocks.Mockprogress
	secretsmanager *mocks.MocksecretsManager
	deployer       *mocks.MockpipelineDeployer
	ws             *mocks.MockwsPipelineReader
	getter         *mocks.MockpipelineGetter
}

func TestDeletePipelineOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		callMocks      func(m deletePipelineMocks)

		wantedError error
	}{
		"happy path with name flag": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.getter.EXPECT().GetPipeline(testPipelineName).Return(&codepipeline.Pipeline{Name: testPipelineName}, nil)
			},
			wantedError: nil,
		},

		"flag-indicated pipeline doesn't exist among deployed pipelines": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.getter.EXPECT().GetPipeline(testPipelineName).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("get pipeline 'honkpipes': some error"),
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

			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)
			mocks := deletePipelineMocks{
				getter: mockPipelineGetter,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					appName: tc.inAppName,
					name:    tc.inPipelineName,
				},
				pipelineSvc: mockPipelineGetter,
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
					gomock.Any(),
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
					skipConfirmation: tc.skipConfirmation,
					appName:          tc.inAppName,
					name:             tc.inPipelineName,
				},
				prompt: mockPrompt,
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
		"skips delete secret confirmation (and deletion attempt) if there is no secret": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DeleteSecret(gomock.Any()).Times(0),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(testPipelineName).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

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
					shouldDeleteSecret: tc.deleteSecret,
					appName:            tc.inAppName,
					name:               tc.inPipelineName,
				},
				PipelineSecret:   tc.inPipelineSecret,
				secretsmanager:   mockSecretsManager,
				pipelineDeployer: mockDeployer,
				ws:               mockWorkspace,
				prog:             mockProg,
				prompt:           mockPrompter,
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
