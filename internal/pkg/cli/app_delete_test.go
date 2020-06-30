// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteAppOpts_Validate(t *testing.T) {
	const mockAppName = "phonetool"
	tests := map[string]struct {
		name string

		want error
	}{
		"should return error if not in a workspace": {
			name: "",
			want: errNoAppInWorkspace,
		},
		"should return nil if app name is set": {
			name: mockAppName,
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						appName: test.name,
					},
				},
			}

			got := opts.Validate()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteAppOpts_Ask(t *testing.T) {
	const mockAppName = "phonetool"
	var mockPrompter *mocks.Mockprompter
	mockError := errors.New("some error")
	tests := map[string]struct {
		skipConfirmation bool

		setupMocks func(ctrl *gomock.Controller)

		want error
	}{
		"return nil if skipConfirmation is enabled": {
			skipConfirmation: true,
			setupMocks:       func(ctrl *gomock.Controller) {},
			want:             nil,
		},
		"wrap error returned from prompting": {
			skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, mockAppName),
						deleteAppConfirmHelp,
						gomock.Any()).
					Return(false, mockError)
			},
			want: fmt.Errorf("confirm app deletion: %w", mockError),
		},
		"return error if user cancels operation": {skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, mockAppName),
						deleteAppConfirmHelp,
						gomock.Any()).
					Return(false, nil)
			},
			want: errOperationCancelled,
		},
		"return nil if user confirms": {
			skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, mockAppName),
						deleteAppConfirmHelp,
						gomock.Any()).
					Return(true, nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						appName: mockAppName,
						prompt:  mockPrompter,
					},
					skipConfirmation: test.skipConfirmation,
				},
			}

			got := opts.Ask()

			require.Equal(t, test.want, got)
		})
	}
}

type deleteAppMocks struct {
	spinner         *mocks.Mockprogress
	store           *mocks.Mockstore
	ws              *mocks.MockwsFileDeleter
	sessProvider    *session.Provider
	deployer        *mocks.Mockdeployer
	svcDeleter      *mocks.Mockexecutor
	envDeleter      *mocks.MockaskExecutor
	bucketEmptier   *mocks.MockbucketEmptier
	pipelineDeleter *mocks.MockdeletePipelineRunner
}

func TestDeleteAppOpts_Execute(t *testing.T) {
	const mockAppName = "phonetool"
	mockServices := []*config.Service{
		{
			Name: "webapp",
		},
	}
	mockEnvs := []*config.Environment{
		{
			Name: "staging",
		},
	}
	mockApp := &config.Application{
		Name: "badgoose",
	}
	mockResources := []*stack.AppRegionalResources{
		{
			Region:   "us-west-2",
			S3Bucket: "goose-bucket",
		},
	}
	tests := map[string]struct {
		appName    string
		setupMocks func(mocks deleteAppMocks)

		wantedError error
	}{
		"happy path": {
			appName: mockAppName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// deleteSvcs
					mocks.store.EXPECT().ListServices(mockAppName).Return(mockServices, nil),
					mocks.svcDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.store.EXPECT().ListEnvironments(mockAppName).Return(mockEnvs, nil),
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(deleteAppCleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg)),

					// delete pipeline
					mocks.pipelineDeleter.EXPECT().Run().Return(nil),

					// deleteAppResources
					mocks.spinner.EXPECT().Start(deleteAppResourcesStartMsg),
					mocks.deployer.EXPECT().DeleteApp(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppResourcesStopMsg)),

					// deleteAppConfigs
					mocks.spinner.EXPECT().Start(deleteAppConfigStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppConfigStopMsg)),

					// deleteWs
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName)),
					mocks.ws.EXPECT().DeleteWorkspaceFile().Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(fmt.Sprintf(fmtDeleteAppWsStopMsg, workspace.SummaryFileName))),
				)
			},
			wantedError: nil,
		},
		"when pipeline manifest does not exist": {
			appName: mockAppName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// deleteSvcs
					mocks.store.EXPECT().ListServices(mockAppName).Return(mockServices, nil),
					mocks.svcDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.store.EXPECT().ListEnvironments(mockAppName).Return(mockEnvs, nil),
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(deleteAppCleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg)),

					// delete pipeline
					mocks.pipelineDeleter.EXPECT().Run().Return(workspace.ErrNoPipelineInWorkspace),

					// deleteAppResources
					mocks.spinner.EXPECT().Start(deleteAppResourcesStartMsg),
					mocks.deployer.EXPECT().DeleteApp(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppResourcesStopMsg)),

					// deleteAppConfigs
					mocks.spinner.EXPECT().Start(deleteAppConfigStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppConfigStopMsg)),

					// deleteWs
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName)),
					mocks.ws.EXPECT().DeleteWorkspaceFile().Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(fmt.Sprintf(fmtDeleteAppWsStopMsg, workspace.SummaryFileName))),
				)
			},
			wantedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSpinner := mocks.NewMockprogress(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsFileDeleter(ctrl)
			mockSession := session.NewProvider()
			mockDeployer := mocks.NewMockdeployer(ctrl)

			mockBucketEmptier := mocks.NewMockbucketEmptier(ctrl)
			mockGetBucketEmptier := func(session *awssession.Session) bucketEmptier {
				return mockBucketEmptier
			}

			// The following three sets of mocks are to avoid having to go through
			// mocking all the intermediary steps in calling Execute on DeleteAppOpts,
			// DeleteEnvOpts, and DeletePipelineOpts. It allows us to instead simply
			// test if the deletion of those resources succeeded or failed.
			mockExecutor := mocks.NewMockexecutor(ctrl)
			mockExecutorProvider := func(appName string) (executor, error) {
				return mockExecutor, nil
			}

			mockAskExecutor := mocks.NewMockaskExecutor(ctrl)
			mockAskExecutorProvider := func(envName, envProfile string) (askExecutor, error) {
				return mockAskExecutor, nil
			}

			mockRunner := mocks.NewMockdeletePipelineRunner(ctrl)
			mockRunnerProvider := func() (deletePipelineRunner, error) {
				return mockRunner, nil
			}

			mocks := deleteAppMocks{
				spinner:         mockSpinner,
				store:           mockStore,
				ws:              mockWorkspace,
				sessProvider:    mockSession,
				deployer:        mockDeployer,
				svcDeleter:      mockExecutor,
				envDeleter:      mockAskExecutor,
				bucketEmptier:   mockBucketEmptier,
				pipelineDeleter: mockRunner,
			}
			test.setupMocks(mocks)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						appName: mockAppName,
					},
				},
				spinner:              mockSpinner,
				store:                mockStore,
				ws:                   mockWorkspace,
				sessProvider:         mockSession,
				cfn:                  mockDeployer,
				s3:                   mockGetBucketEmptier,
				executor:             mockExecutorProvider,
				askExecutor:          mockAskExecutorProvider,
				deletePipelineRunner: mockRunnerProvider,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			require.Equal(t, test.wantedError, err)
		})
	}
}
