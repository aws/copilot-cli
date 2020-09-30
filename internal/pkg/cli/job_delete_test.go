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
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deleteJobMocks struct {
	store          *mocks.Mockstore
	secretsmanager *mocks.MocksecretsManager
	sessProvider   *sessions.Provider
	appCFN         *mocks.MockjobRemoverFromApp
	spinner        *mocks.Mockprogress
	jobCFN         *mocks.MockjobDeleter
	ecr            *mocks.MockimageRemover
}

func TestDeleteJobOpts_Execute(t *testing.T) {
	mockJobName := "resizer"
	mockEnvName := "test"
	mockAppName := "badgoose"
	mockEnv := &config.Environment{
		App:            mockAppName,
		Name:           mockEnvName,
		ManagerRoleARN: "some-arn",
		Region:         "us-west-2",
	}
	mockEnvs := []*config.Environment{mockEnv}
	mockApp := &config.Application{
		Name: mockAppName,
	}

	mockRepo := fmt.Sprintf("%s/%s", mockAppName, mockJobName)
	testError := errors.New("some error")

	tests := map[string]struct {
		inAppName string
		inEnvName string
		inJobName string

		setupMocks func(mocks deleteJobMocks)

		wantedError error
	}{
		"happy path with no environment passed in as flag": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().ListEnvironments(gomock.Eq(mockAppName)).Times(1).Return(mockEnvs, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobDeleteStart, mockJobName, mockEnvName)),
					mocks.jobCFN.EXPECT().DeleteJob(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtJobDeleteComplete, mockJobName, mockEnvName)),
					// emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeJobFromApp
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobDeleteResourcesStart, mockJobName, mockAppName)),
					mocks.appCFN.EXPECT().RemoveJobFromApp(mockApp, mockJobName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtJobDeleteResourcesComplete, mockJobName, mockAppName)),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteJob(mockAppName, mockJobName).Return(nil),
				)
			},
			wantedError: nil,
		},
		// A job can be deployed to multiple
		// environments - and deleting it in one
		// should not delete it form the entire app.
		"happy path with environment passed in as flag": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobDeleteStart, mockJobName, mockEnvName)),
					mocks.jobCFN.EXPECT().DeleteJob(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtJobDeleteComplete, mockJobName, mockEnvName)),

					// It should **not** emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(gomock.Any()).Return(nil).Times(0),

					// It should **not** removeJobFromApp
					mocks.appCFN.EXPECT().RemoveJobFromApp(gomock.Any(), gomock.Any()).Return(nil).Times(0),

					// It should **not** deleteSSMParam
					mocks.store.EXPECT().DeleteJob(gomock.Any(), gomock.Any()).Return(nil).Times(0),
				)
			},
			wantedError: nil,
		},
		"errors when deleting stack": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobDeleteStart, mockJobName, mockEnvName)),
					mocks.jobCFN.EXPECT().DeleteJob(gomock.Any()).Return(testError),
					mocks.spinner.EXPECT().Stop(log.Serrorf(fmtJobDeleteFailed, mockJobName, mockEnvName, testError)),
				)
			},
			wantedError: testError,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockSession := sessions.NewProvider()
			mockAppCFN := mocks.NewMockjobRemoverFromApp(ctrl)
			mockJobCFN := mocks.NewMockjobDeleter(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockImageRemover := mocks.NewMockimageRemover(ctrl)
			mockGetJobCFN := func(_ *session.Session) jobDeleter {
				return mockJobCFN
			}

			mockGetImageRemover := func(_ *session.Session) imageRemover {
				return mockImageRemover
			}
			mocks := deleteJobMocks{
				store:          mockstore,
				secretsmanager: mockSecretsManager,
				sessProvider:   mockSession,
				appCFN:         mockAppCFN,
				spinner:        mockSpinner,
				jobCFN:         mockJobCFN,
				ecr:            mockImageRemover,
			}

			test.setupMocks(mocks)

			opts := deleteJobOpts{
				deleteJobVars: deleteJobVars{
					appName: test.inAppName,
					name:    test.inJobName,
					envName: test.inEnvName,
				},
				store:     mockstore,
				sess:      mockSession,
				spinner:   mockSpinner,
				appCFN:    mockAppCFN,
				getJobCFN: mockGetJobCFN,
				getECR:    mockGetImageRemover,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
