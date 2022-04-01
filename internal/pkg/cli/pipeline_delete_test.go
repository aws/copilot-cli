// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	sdkSecretsmanager "github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deletePipelineMocks struct {
	prompt                 *mocks.Mockprompter
	prog                   *mocks.Mockprogress
	secretsmanager         *mocks.MocksecretsManager
	deployer               *mocks.MockpipelineDeployer
	ws                     *mocks.MockwsPipelineGetter
	store                  *mocks.Mockstore
	codepipeline           *mocks.MockpipelineGetter
	sel                    *mocks.MockcodePipelineSelector
	deployedPipelineLister *mocks.MockdeployedPipelineLister
}

func TestDeletePipelineOpts_Ask(t *testing.T) {
	const (
		testAppName      = "badgoose"
		testPipelineName = "pipeline-badgoose-honkpipes"
	)
	testCases := map[string]struct {
		skipConfirmation bool
		inAppName        string
		inPipelineName   string

		callMocks          func(m deletePipelineMocks)
		wantedAppName      string
		wantedPipelineName string
		wantedError        error
	}{
		"prompts for app name if empty": {
			inPipelineName:   testPipelineName,
			skipConfirmation: true,

			callMocks: func(m deletePipelineMocks) {
				m.sel.EXPECT().Application(pipelineDeleteAppNamePrompt, pipelineDeleteAppNameHelpPrompt).Return(testAppName, nil)
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(testAppName).Return([]deploy.Pipeline{
					{
						Name: testPipelineName,
					},
				}, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
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
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(testAppName).Return([]deploy.Pipeline{}, nil)
			},

			wantedAppName: testAppName,
			wantedError:   errors.New("validate pipeline name badPipelineName: cannot find pipeline named badPipelineName"),
		},
		"gets name of legacy pipeline": {
			skipConfirmation: true,
			inAppName:        testAppName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), testAppName).Return(deploy.Pipeline{
					Name:     testPipelineName,
					IsLegacy: true,
				}, nil)
			},
			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedError:        nil,
		},
		"error getting pipeline": {
			skipConfirmation: true,
			inAppName:        testAppName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), testAppName).Return(deploy.Pipeline{}, errors.New("some error"))
			},

			wantedAppName: testAppName,
			wantedError:   errors.New("select deployed pipelines: some error"),
		},
		"skip confirmation works": {
			skipConfirmation: true,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,

			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(testAppName).Return([]deploy.Pipeline{
					{
						Name: testPipelineName,
					},
				}, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
			wantedError:        nil,
		},
		"delete confirmation works": {
			skipConfirmation: false,
			inAppName:        testAppName,
			inPipelineName:   testPipelineName,
			callMocks: func(m deletePipelineMocks) {
				m.store.EXPECT().GetApplication(testAppName).Return(nil, nil)
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(testAppName).Return([]deploy.Pipeline{
					{
						Name: testPipelineName,
					},
				}, nil)
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(pipelineDeleteConfirmPrompt, testPipelineName, testAppName),
					pipelineDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, nil)
			},

			wantedAppName:      testAppName,
			wantedPipelineName: testPipelineName,
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
				codepipeline:           mockPipelineGetter,
				prompt:                 mockPrompt,
				ws:                     mockWorkspace,
				store:                  mockStore,
				sel:                    mockSel,
				deployedPipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
			}

			tc.callMocks(mocks)

			opts := &deletePipelineOpts{
				deletePipelineVars: deletePipelineVars{
					skipConfirmation: tc.skipConfirmation,
					appName:          tc.inAppName,
					name:             tc.inPipelineName,
				},
				codepipeline:           mockPipelineGetter,
				prompt:                 mockPrompt,
				ws:                     mockWorkspace,
				store:                  mockStore,
				sel:                    mockSel,
				deployedPipelineLister: mocks.deployedPipelineLister,
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
			}
		})
	}
}

func TestDeletePipelineOpts_Execute(t *testing.T) {
	const (
		testAppName        = "badgoose"
		testPipelineName   = "pipeline-badgoose-honkpipes"
		testPipelineSecret = "github-token-badgoose-honkpipes"
	)
	targetPipeline := deploy.Pipeline{
		Name:     testPipelineName,
		AppName:  testAppName,
		IsLegacy: true,
	}
	mockTime := time.Now()
	mockResp := &secretsmanager.DescribeSecretOutput{
		CreatedDate: aws.Time(mockTime),
		Name:        aws.String(testPipelineSecret),
		Tags: []*sdkSecretsmanager.Tag{
			{
				Key:   aws.String(deploy.AppTagKey),
				Value: aws.String(mockTime.UTC().Format(time.UnixDate)),
			},
		},
	}
	mockBadResp := &secretsmanager.DescribeSecretOutput{
		CreatedDate: aws.Time(mockTime),
		Name:        aws.String(testPipelineSecret),
		Tags: []*sdkSecretsmanager.Tag{
			{
				Key:   aws.String("someOtherKey"),
				Value: aws.String(mockTime.UTC().Format(time.UnixDate)),
			},
		},
	}
	testError := errors.New("some error")
	testCases := map[string]struct {
		deleteSecret   bool
		inAppName      string
		inPipelineName string

		setupMocks func(mocks deletePipelineMocks)

		wantedError error
	}{
		"skips delete secret confirmation (and deletion attempt) if there is no secret": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(nil, &secretsmanager.ErrSecretNotFound{}),
					mocks.secretsmanager.EXPECT().DeleteSecret(gomock.Any()).Times(0),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},
		"skips secret deletion if secret found but tags don't match": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(mockBadResp, nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},
		"wraps error from DescribeSecret": {
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("describe secret %s: some error", testPipelineSecret),
		},
		"skips delete secret confirmation when flag is specified": {
			deleteSecret:   true,
			inAppName:      testAppName,
			inPipelineName: testPipelineName,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(mockResp, nil),
					// no confirmation prompt for deleting secret
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"asks for confirmation when delete secret flag is not specified": {
			deleteSecret:   false,
			inAppName:      testAppName,
			inPipelineName: testPipelineName,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(mockResp, nil),
					mocks.prompt.EXPECT().Confirm(
						fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, testPipelineSecret, testPipelineName),
						pipelineDeleteSecretConfirmHelp,
					).Times(1).Return(true, nil),
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"does not delete secret if user does not confirm": {
			deleteSecret:   false,
			inAppName:      testAppName,
			inPipelineName: testPipelineName,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(mockResp, nil),
					mocks.prompt.EXPECT().Confirm(
						fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, testPipelineSecret, testPipelineName),
						pipelineDeleteSecretConfirmHelp,
					).Times(1).Return(false, nil),

					// does not delete secret
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Times(0),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Times(1).Return(nil),
					mocks.prog.EXPECT().Stop(log.Ssuccessf(fmtDeletePipelineComplete, testPipelineName, testAppName)),
				)
			},
			wantedError: nil,
		},

		"error when deleting stack": {
			deleteSecret:   true,
			inAppName:      testAppName,
			inPipelineName: testPipelineName,

			setupMocks: func(mocks deletePipelineMocks) {
				gomock.InOrder(
					mocks.secretsmanager.EXPECT().DescribeSecret(testPipelineSecret).Return(mockResp, nil),
					mocks.secretsmanager.EXPECT().DeleteSecret(testPipelineSecret).Return(nil),
					mocks.prog.EXPECT().Start(fmt.Sprintf(fmtDeletePipelineStart, testPipelineName, testAppName)),
					mocks.deployer.EXPECT().DeletePipeline(targetPipeline).Times(1).Return(testError),
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
				secretsmanager:   mockSecretsManager,
				pipelineDeployer: mockDeployer,
				ws:               mockWorkspace,
				prog:             mockProg,
				prompt:           mockPrompter,
				targetPipeline:   &targetPipeline,
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
