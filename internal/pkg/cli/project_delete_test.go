// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteProjectOpts_Validate(t *testing.T) {
	const mockProjectName = "phonetool"
	tests := map[string]struct {
		projectName string

		want error
	}{
		"should return error if not in a workspace": {
			projectName: "",
			want:        errNoAppInWorkspace,
		},
		"should return nil if project name is set": {
			projectName: mockProjectName,
			want:        nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						appName: test.projectName,
					},
				},
			}

			got := opts.Validate()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteProjectOpts_Ask(t *testing.T) {
	const mockProjectName = "phonetool"
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
		"echo error returned from prompting": {
			skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
						gomock.Any()).
					Return(false, mockError)
			},
			want: mockError,
		},
		"return error if user cancels operation": {skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
						gomock.Any()).
					Return(false, nil)
			},
			want: errOperationCancelled,
		},
		"return nil if user confirms": {skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
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
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						appName: mockProjectName,
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

func TestDeleteProjectOpts_DeleteApps(t *testing.T) {
	var mockStore *mocks.Mockstore
	const mockProjectName = "phonetool"
	mockError := errors.New("some error")
	tests := map[string]struct {
		setupMocks func(ctrl *gomock.Controller)
		want       error
	}{
		"return error is listing applications fails": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockstore(ctrl)

				mockStore.EXPECT().
					ListServices(mockProjectName).
					Return(nil, mockError)
			},
			want: mockError,
		},
		"return nil if no apps returned from listing applications": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockstore(ctrl)

				mockStore.EXPECT().
					ListServices(mockProjectName).
					Return(nil, nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						appName: mockProjectName,
					},
				},
				store: mockStore,
			}

			got := opts.deleteApps()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteProjectOpts_EmptyS3Bucket(t *testing.T) {
	const mockProjectName = "phonetool"
	mockError := errors.New("some error")
	var mockStore *mocks.Mockstore

	tests := map[string]struct {
		setupMocks func(ctrl *gomock.Controller)
		want       error
	}{
		"return error is listing applications fails": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockstore(ctrl)

				mockStore.EXPECT().
					ListServices(mockProjectName).
					Return(nil, mockError)
			},
			want: mockError,
		},
		"return nil if no apps returned from listing applications": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockstore(ctrl)

				mockStore.EXPECT().
					ListServices(mockProjectName).
					Return(nil, nil)
			},
			want: nil,
		},
		// TODO: add more tests when app deletion workflow is inline mockable (provider pattern?)
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						appName: mockProjectName,
					},
				},
				store: mockStore,
			}

			got := opts.deleteApps()

			require.Equal(t, test.want, got)
		})
	}
}

type deleteProjectMocks struct {
	spinner         *mocks.Mockprogress
	store           *mocks.Mockstore
	ws              *mocks.MockworkspaceDeleter
	sessProvider    *session.Provider
	deployer        *mocks.Mockdeployer
	appDeleter      *mocks.Mockexecutor
	envDeleter      *mocks.MockaskExecutor
	bucketEmptier   *mocks.MockbucketEmptier
	pipelineDeleter *mocks.MockdeletePipelineRunner
}

func TestDeleteProjectOpts_Execute(t *testing.T) {
	const mockProjectName = "phonetool"
	mockApps := []*config.Service{
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
		&stack.AppRegionalResources{
			Region:   "us-west-2",
			S3Bucket: "goose-bucket",
		},
	}
	tests := map[string]struct {
		projectName string
		setupMocks  func(mocks deleteProjectMocks)

		wantedError error
	}{
		"happy path": {

			projectName: mockProjectName,
			setupMocks: func(mocks deleteProjectMocks) {
				gomock.InOrder(
					// deleteApps
					mocks.store.EXPECT().ListServices(mockProjectName).Return(mockApps, nil),
					mocks.appDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.store.EXPECT().ListEnvironments(mockProjectName).Return(mockEnvs, nil),
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(cleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(cleanResourcesStopMsg)),

					// deleteProjectPipline
					mocks.pipelineDeleter.EXPECT().Run().Return(nil),

					// deleteProjectResources
					mocks.spinner.EXPECT().Start(deleteProjectResourcesStartMsg),
					mocks.deployer.EXPECT().DeleteApp(mockProjectName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteProjectResourcesStopMsg)),

					// deleteProjectParams
					mocks.spinner.EXPECT().Start(deleteProjectParamsStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockProjectName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteProjectParamsStopMsg)),

					// deleteLocalWorkspace
					mocks.spinner.EXPECT().Start(deleteLocalWsStartMsg),
					mocks.ws.EXPECT().DeleteAll().Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteLocalWsStopMsg)),
				)
			},
			wantedError: nil,
		},
		"when pipeline manifest does not exist": {

			projectName: mockProjectName,
			setupMocks: func(mocks deleteProjectMocks) {
				gomock.InOrder(
					// deleteApps
					mocks.store.EXPECT().ListServices(mockProjectName).Return(mockApps, nil),
					mocks.appDeleter.EXPECT().Execute().Return(nil),

					// deleteEnvs
					mocks.store.EXPECT().ListEnvironments(mockProjectName).Return(mockEnvs, nil),
					mocks.envDeleter.EXPECT().Ask().Return(nil),
					mocks.envDeleter.EXPECT().Execute().Return(nil),

					// emptyS3bucket
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(mockApp, nil),
					mocks.deployer.EXPECT().GetRegionalAppResources(mockApp).Return(mockResources, nil),
					mocks.spinner.EXPECT().Start(cleanResourcesStartMsg),
					mocks.bucketEmptier.EXPECT().EmptyBucket(mockResources[0].S3Bucket).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(cleanResourcesStopMsg)),

					// deleteProjectPipline
					mocks.pipelineDeleter.EXPECT().Run().Return(workspace.ErrNoPipelineInWorkspace),

					// deleteProjectResources
					mocks.spinner.EXPECT().Start(deleteProjectResourcesStartMsg),
					mocks.deployer.EXPECT().DeleteApp(mockProjectName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteProjectResourcesStopMsg)),

					// deleteProjectParams
					mocks.spinner.EXPECT().Start(deleteProjectParamsStartMsg),
					mocks.store.EXPECT().DeleteApplication(mockProjectName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteProjectParamsStopMsg)),

					// deleteLocalWorkspace
					mocks.spinner.EXPECT().Start(deleteLocalWsStartMsg),
					mocks.ws.EXPECT().DeleteAll().Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccess(deleteLocalWsStopMsg)),
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
			mockWorkspace := mocks.NewMockworkspaceDeleter(ctrl)
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

			mocks := deleteProjectMocks{
				spinner:         mockSpinner,
				store:           mockStore,
				ws:              mockWorkspace,
				sessProvider:    mockSession,
				deployer:        mockDeployer,
				appDeleter:      mockExecutor,
				envDeleter:      mockAskExecutor,
				bucketEmptier:   mockBucketEmptier,
				pipelineDeleter: mockRunner,
			}
			test.setupMocks(mocks)

			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						appName: mockProjectName,
					},
				},
				spinner:                      mockSpinner,
				store:                        mockStore,
				ws:                           mockWorkspace,
				sessProvider:                 mockSession,
				deployer:                     mockDeployer,
				getBucketEmptier:             mockGetBucketEmptier,
				executorProvider:             mockExecutorProvider,
				askExecutorProvider:          mockAskExecutorProvider,
				deletePipelineRunnerProvider: mockRunnerProvider,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			require.Equal(t, test.wantedError, err)
		})
	}
}
