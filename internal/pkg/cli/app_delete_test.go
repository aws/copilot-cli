// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deleteAppMocks struct {
	spinner         *mocks.Mockprogress
	store           *mocks.Mockstore
	codepipeline    *mocks.MockdeployedPipelineLister
	ws              *mocks.MockwsAppManagerDeleter
	sessProvider    *sessions.Provider
	deployer        *mocks.Mockdeployer
	svcDeleter      *mocks.Mockexecutor
	jobDeleter      *mocks.Mockexecutor
	envDeleter      *mocks.MockexecuteAsker
	taskDeleter     *mocks.Mockexecutor
	bucketEmptier   *mocks.MockbucketEmptier
	pipelineDeleter *mocks.Mockexecutor
	prompt          *mocks.Mockprompter
	sel             *mocks.MockappSelector
}

func TestDeleteAppOpts_Ask(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		inAppName        string
		skipConfirmation bool
		setUpMocks       func(m *deleteAppMocks)

		want error
	}{
		"return nil if skipConfirmation is enabled and appName is provided": {
			inAppName:        "phonetool",
			skipConfirmation: true,
			setUpMocks: func(m *deleteAppMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
		},
		"return an error if user provided app doesn't exist in SSM": {
			inAppName:        "phonetool",
			skipConfirmation: true,
			setUpMocks: func(m *deleteAppMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(nil, errors.New("couldn't find an application named phonetool in account 555555555 and region us-west-2"))
			},
			want: fmt.Errorf("couldn't find an application named phonetool in account 555555555 and region us-west-2"),
		},
		"wrap error returned from prompting": {
			inAppName: "phonetool",
			setUpMocks: func(m *deleteAppMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.prompt.EXPECT().
					Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, "phonetool"),
						deleteAppConfirmHelp,
						gomock.Any()).
					Return(false, mockError)
			},
			want: fmt.Errorf("confirm app deletion: %w", mockError),
		},
		"return error if user cancels operation": {
			setUpMocks: func(m *deleteAppMocks) {
				m.sel.EXPECT().Application(appDeleteNamePrompt, "").Return("phonetool", nil)
				m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, "phonetool"),
					deleteAppConfirmHelp,
					gomock.Any()).
					Return(false, nil)
			},
			want: errOperationCancelled,
		},
		"select from list of apps and user confirms": {
			setUpMocks: func(m *deleteAppMocks) {
				m.sel.EXPECT().Application(appDeleteNamePrompt, "").Return("phonetool", nil)
				m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtDeleteAppConfirmPrompt, "phonetool"),
					deleteAppConfirmHelp,
					gomock.Any()).
					Return(true, nil)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockPrompter := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockappSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			m := &deleteAppMocks{
				sel:    mockSel,
				prompt: mockPrompter,
				store:  mockStore,
			}
			test.setUpMocks(m)
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					name:             test.inAppName,
					skipConfirmation: test.skipConfirmation,
				},
				prompt: mockPrompter,
				sel:    mockSel,
				store:  mockStore,
			}

			got := opts.Ask()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteAppOpts_Execute(t *testing.T) {
	const mockAppName = "phonetool"
	mockServices := []*config.Workload{
		{
			Name: "webapp",
		},
		{
			Name: "backend",
		},
	}
	mockJobs := []*config.Workload{
		{
			Name: "mailer",
		},
		{
			Name: "bailer",
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
	mockPipelines := []deploy.Pipeline{
		{
			AppName:      "badgoose",
			ResourceName: "pipeline1",
			IsLegacy:     false,
		},
		{
			AppName:      "badgoose",
			ResourceName: "pipeline2",
			IsLegacy:     false,
		},
	}
	mockResources := []*stack.AppRegionalResources{
		{
			Region:   "us-west-2",
			S3Bucket: "goose-bucket",
		},
	}
	mockTaskStacks := []deploy.TaskStackInfo{
		{
			StackName: "task-db-migrate",
			App:       "badgoose",
			Env:       "staging",
		},
	}
	tests := map[string]struct {
		appName    string
		setupMocks func(mocks deleteAppMocks)

		wantedError error
	}{
		"success deleting all the resources along with workspace summary": {
			appName: mockAppName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// delete pipelines
					mocks.codepipeline.EXPECT().ListDeployedPipelines(mockAppName).Return(mockPipelines, nil),
					mocks.pipelineDeleter.EXPECT().Execute().Return(nil).Times(2),

					// deleteSvcs
					mocks.store.EXPECT().ListServices(mockAppName).Return(mockServices, nil),
					mocks.svcDeleter.EXPECT().Execute().Return(nil).Times(2),

					// deleteJobs
					mocks.store.EXPECT().ListJobs(mockAppName).Return(mockJobs, nil),
					mocks.jobDeleter.EXPECT().Execute().Return(nil).Times(2),

					// listEnvs
					mocks.store.EXPECT().ListEnvironments(mockAppName).Return(mockEnvs, nil),

					// deleteTasks
					mocks.deployer.EXPECT().ListTaskStacks(mockAppName, mockEnvs[0].Name).Return(mockTaskStacks, nil),
					mocks.taskDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(deleteAppCleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg)),

					// deleteAppResources
					mocks.deployer.EXPECT().DeleteApp(mockAppName).Return(nil),

					// deleteAppConfigs
					mocks.spinner.EXPECT().Start(deleteAppConfigStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppConfigStopMsg)),

					// deleteWs
					mocks.ws.EXPECT().Summary().Return(&workspace.Summary{Application: mockAppName}, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName)),
					mocks.ws.EXPECT().DeleteWorkspaceFile().Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(fmt.Sprintf(fmtDeleteAppWsStopMsg, workspace.SummaryFileName))),
				)
			},
		},
		"skip deleting if failed to fetch workspace summary": {
			appName: mockAppName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// delete pipelines
					mocks.codepipeline.EXPECT().ListDeployedPipelines(mockAppName).Return(mockPipelines, nil),
					mocks.pipelineDeleter.EXPECT().Execute().Return(nil).Times(2),

					// deleteSvcs
					mocks.store.EXPECT().ListServices(mockAppName).Return(mockServices, nil),
					mocks.svcDeleter.EXPECT().Execute().Return(nil).Times(2),

					// deleteJobs
					mocks.store.EXPECT().ListJobs(mockAppName).Return(mockJobs, nil),
					mocks.jobDeleter.EXPECT().Execute().Return(nil).Times(2),

					// listEnvs
					mocks.store.EXPECT().ListEnvironments(mockAppName).Return(mockEnvs, nil),

					// deleteTasks
					mocks.deployer.EXPECT().ListTaskStacks(mockAppName, mockEnvs[0].Name).Return(mockTaskStacks, nil),
					mocks.taskDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(deleteAppCleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg)),

					// deleteAppResources
					mocks.deployer.EXPECT().DeleteApp(mockAppName).Return(nil),

					// deleteAppConfigs
					mocks.spinner.EXPECT().Start(deleteAppConfigStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteAppConfigStopMsg)),

					// deleteWs
					mocks.ws.EXPECT().Summary().Return(nil, errors.New("some error")),
				)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSpinner := mocks.NewMockprogress(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsAppManagerDeleter(ctrl)
			mockSession := sessions.ImmutableProvider()
			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockPipelineLister := mocks.NewMockdeployedPipelineLister(ctrl)

			mockBucketEmptier := mocks.NewMockbucketEmptier(ctrl)
			mockGetBucketEmptier := func(session *session.Session) bucketEmptier {
				return mockBucketEmptier
			}

			// The following three sets of mocks are to avoid having to go through
			// mocking all the intermediary steps in calling Execute on DeleteAppOpts,
			// DeleteEnvOpts, and DeletePipelineOpts. It allows us to instead simply
			// test if the deletion of those resources succeeded or failed.
			mockSvcDeleteExecutor := mocks.NewMockexecutor(ctrl)
			mockSvcExecutorProvider := func(appName, svcName string) (executor, error) {
				return mockSvcDeleteExecutor, nil
			}
			mockJobDeleteExecutor := mocks.NewMockexecutor(ctrl)
			mockJobExecutorProvider := func(appName, jobName string) (executor, error) {
				return mockJobDeleteExecutor, nil
			}
			mockTaskDeleteExecutor := mocks.NewMockexecutor(ctrl)
			mockTaskDeleteProvider := func(appName, envName, taskName string) (executor, error) {
				return mockTaskDeleteExecutor, nil
			}
			mockEnvDeleteExecutor := mocks.NewMockexecuteAsker(ctrl)
			mockAskExecutorProvider := func(appName, envName string) (executeAsker, error) {
				return mockEnvDeleteExecutor, nil
			}

			mockPipelineDeleteExecutor := mocks.NewMockexecutor(ctrl)
			mockPipelineExecutorProvider := func(appName, pipelineName string) (executor, error) {
				return mockPipelineDeleteExecutor, nil
			}

			mocks := deleteAppMocks{
				spinner:         mockSpinner,
				store:           mockStore,
				ws:              mockWorkspace,
				sessProvider:    mockSession,
				deployer:        mockDeployer,
				codepipeline:    mockPipelineLister,
				svcDeleter:      mockSvcDeleteExecutor,
				jobDeleter:      mockJobDeleteExecutor,
				envDeleter:      mockEnvDeleteExecutor,
				taskDeleter:     mockTaskDeleteExecutor,
				bucketEmptier:   mockBucketEmptier,
				pipelineDeleter: mockPipelineDeleteExecutor,
			}
			test.setupMocks(mocks)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					name: mockAppName,
				},
				spinner: mockSpinner,
				store:   mockStore,
				existingWorkSpace: func() (wsAppManagerDeleter, error) {
					return mockWorkspace, nil
				},
				pipelineLister:         mockPipelineLister,
				sessProvider:           mockSession,
				cfn:                    mockDeployer,
				s3:                     mockGetBucketEmptier,
				svcDeleteExecutor:      mockSvcExecutorProvider,
				jobDeleteExecutor:      mockJobExecutorProvider,
				envDeleteExecutor:      mockAskExecutorProvider,
				taskDeleteExecutor:     mockTaskDeleteProvider,
				pipelineDeleteExecutor: mockPipelineExecutorProvider,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			require.Equal(t, test.wantedError, err)
		})
	}
}
