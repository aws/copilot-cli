// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
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
	ws             *mocks.MockwsPipelineGetter
	store          *mocks.Mockstore
	codepipeline   *mocks.MockpipelineGetter
	sel            *mocks.MockcodePipelineSelector
}

func TestDeletePipelineOpts_Ask(t *testing.T) {
	mockPipelineManifest := &manifest.Pipeline{
		Name:    testPipelineName,
		Version: 1,
		Source: &manifest.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"access_token_secret": "github-token-badgoose-backend",
				"repository":          "aws/somethingCool",
				"branch":              "main",
			},
		},
	}
	mockPipelineManifestWithoutSecret := &manifest.Pipeline{
		Name:    testPipelineName,
		Version: 1,
		Source: &manifest.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository": "aws/somethingCool",
				"branch":     "main",
			},
		},
	}
	testCases := map[string]struct {
		skipConfirmation bool
		inAppName        string
		inPipelineName   string

		callMocks          func(m deletePipelineMocks)
		wantedAppName      string
		wantedPipelineName string
		wantedSecret       string
		wantedError        error
	}{
		"prompts for app name if empty": {
			inPipelineName:   testPipelineName,
			skipConfirmation: true,

			callMocks: func(m deletePipelineMocks) {
				m.sel.EXPECT().Application(pipelineDeleteAppNamePrompt, pipelineDeleteAppNameHelpPrompt).Return(testAppName, nil)
				m.codepipeline.EXPECT().GetPipeline(testPipelineName).Return(nil, nil)
				m.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifest, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedSecret:       "github-token-badgoose-backend",
			wantedError:        nil,
		},
		"errors if passed-in app name invalid": {
			skipConfirmation: true,
			inAppName:        "badAppName",

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication("badAppName").Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"errors if passed-in pipeline name is invalid": {
			skipConfirmation: true,
			inAppName:        testAppName,
			inPipelineName:   "badPipelineName",

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.codepipeline.EXPECT().GetPipeline("badPipelineName").Return(nil, errors.New("some error"))
			},

			wantedAppName: testAppName,
			wantedError:   errors.New("some error"),
		},
		"gets name of pipeline; gets legacy secret": {
			skipConfirmation: true,
			inAppName:        testAppName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), gomock.Any()).Return(testPipelineName, nil)
				m.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifest, nil)
			},
			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedSecret:       "github-token-badgoose-backend",
			wantedError:        nil,
		},
		"gets name of pipeline; no legacy secret": {
			skipConfirmation: true,
			inAppName:        testAppName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), gomock.Any()).Return(testPipelineName, nil)
				m.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifestWithoutSecret, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedSecret:       "",
			wantedError:        nil,
		},
		"error getting pipeline": {
			skipConfirmation: true,
			inAppName:        testAppName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},

			wantedAppName: testAppName,
			wantedSecret:  "",
			wantedError:   errors.New("select deployed pipelines: some error"),
		},
		"skip confirmation works": {
			skipConfirmation: true,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.codepipeline.EXPECT().GetPipeline(testPipelineName).Return(nil, nil)
				m.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifest, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedSecret:       "github-token-badgoose-backend",
			wantedError:        nil,
		},

		"delete confirmation works": {
			skipConfirmation: false,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.codepipeline.EXPECT().GetPipeline(testPipelineName).Return(nil, nil)
				m.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifest, nil)
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(pipelineDeleteConfirmPrompt, testPipelineName, testAppName),
					pipelineDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedSecret:       "github-token-badgoose-backend",
			wantedError:        nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineGetter(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockcodePipelineSelector(ctrl)

			mocks := deletePipelineMocks{
				codepipeline: mockPipelineGetter,
				prompt:       mockPrompt,
				ws:           mockWorkspace,
				store:        mockStore,
				sel:          mockSel,
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					skipConfirmation: tc.skipConfirmation,
					appName:          tc.inAppName,
					name:             tc.inPipelineName,
				},
				codepipeline: mockPipelineGetter,
				prompt:       mockPrompt,
				ws:           mockWorkspace,
				store:        mockStore,
				sel:          mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedAppName, opts.appName, "expected app names to match")
				require.Equal(t, tc.wantedPipelineName, opts.name, "expected pipeline names to match")
				require.Equal(t, tc.wantedSecret, opts.ghAccessTokenSecretName, "expected secrets to match")
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
			mockWorkspace := mocks.NewMockwsPipelineGetter(ctrl)

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
				ghAccessTokenSecretName: tc.inPipelineSecret,
				secretsmanager:          mockSecretsManager,
				pipelineDeployer:        mockDeployer,
				ws:                      mockWorkspace,
				prog:                    mockProg,
				prompt:                  mockPrompter,
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
