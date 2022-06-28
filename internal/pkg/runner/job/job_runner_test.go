// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package job

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/runner/job/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestJobRunner_Run(t *testing.T) {

	testCases := map[string]struct {
		MockExecutor func(m *mocks.MockJobExecutor)

		App string
		Env string
		Job string

		MockStackRetriever func(m *mocks.MockStackRetriever)

		wantedError error
	}{

		"missing stack": {
			MockExecutor: func(m *mocks.MockJobExecutor) {
				m.EXPECT().Execute("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job").Return(nil).AnyTimes()
			},
			App: "appname",
			Env: "envname",
			Job: "jobname",
			MockStackRetriever: func(m *mocks.MockStackRetriever) {
				m.EXPECT().StackResources("appname-envname-jobname").Return(nil, fmt.Errorf("Missing Stack Resource"))
			},
			wantedError: fmt.Errorf("describe stack appname-envname-jobname: Missing Stack Resource"),
		},

		"missing statemachine resource": {
			MockExecutor: func(m *mocks.MockJobExecutor) {
				m.EXPECT().Execute("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job").Return(nil).AnyTimes()
			},
			App: "appname",
			Env: "envname",
			Job: "jobname",
			MockStackRetriever: func(m *mocks.MockStackRetriever) {
				m.EXPECT().StackResources("appname-envname-jobname").Return([]*cloudformation.StackResource{
					{
						ResourceType: aws.String("AWS::Lambda::Function"),
					},
				}, nil)
			},
			wantedError: fmt.Errorf("statemachine not found"),
		},

		"failed statemachine execution": {
			MockExecutor: func(m *mocks.MockJobExecutor) {
				m.EXPECT().Execute("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job").Return(fmt.Errorf("ExecutionLimitExceeded"))
			},
			App: "appname",
			Env: "envname",
			Job: "jobname",
			MockStackRetriever: func(m *mocks.MockStackRetriever) {
				m.EXPECT().StackResources("appname-envname-jobname").Return([]*cloudformation.StackResource{
					{
						ResourceType:       aws.String("AWS::StepFunctions::StateMachine"),
						PhysicalResourceId: aws.String("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job"),
					},
				}, nil)
			},
			wantedError: fmt.Errorf("statemachine execution: ExecutionLimitExceeded"),
		},

		"run success": {
			MockExecutor: func(m *mocks.MockJobExecutor) {
				m.EXPECT().Execute("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job").Return(nil)
			},
			App: "appname",
			Env: "envname",
			Job: "jobname",
			MockStackRetriever: func(m *mocks.MockStackRetriever) {
				m.EXPECT().StackResources("appname-envname-jobname").Return([]*cloudformation.StackResource{
					{
						ResourceType:       aws.String("AWS::StepFunctions::StateMachine"),
						PhysicalResourceId: aws.String("arn:aws:states:us-east-1:111111111111:stateMachine:app-env-job"),
					},
				}, nil)
			},
		},
	}

	for name, tc := range testCases {

		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockStackRetriever := mocks.NewMockStackRetriever(ctrl)
			MockJobExecutor := mocks.NewMockJobExecutor(ctrl)

			tc.MockStackRetriever(MockStackRetriever)
			tc.MockExecutor(MockJobExecutor)

			jobRunner := JobRunner{
				Executor:       MockJobExecutor,
				App:            tc.App,
				Env:            tc.Env,
				Job:            tc.Job,
				StackRetriever: MockStackRetriever,
			}

			err := jobRunner.Run()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
