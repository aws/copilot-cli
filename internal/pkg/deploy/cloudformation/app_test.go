// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestDeployApp(t *testing.T) {
	mockTemplate := "mockTemplate"
	mockStackName := "mockStackName"
	mockChangeSetName := "mockChangeSetName"
	// mockError := errors.New("mockError")

	testCases := map[string]struct {
		mockCreateStack                      func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
		mockWaitUntilStackCreateComplete     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
		mockCreateChangeSet                  func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
		mockWaitUntilChangeSetCreateComplete func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
		mockExecuteChangeSet                 func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error)
		mockWaitUntilStackUpdateComplete     func(t *testing.T, in *cloudformation.DescribeStacksInput) error

		wantErr error
	}{
		"should create the stack if one didn't exist already and wait for completion": {
			mockCreateStack: func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])

				return &cloudformation.CreateStackOutput{}, nil
			},
			mockWaitUntilStackCreateComplete: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)

				return nil
			},
		},
		"should create and execute change set if stack already exists": {
			mockCreateStack: func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])

				return nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil)
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, "UPDATE", *in.ChangeSetType)

				return &cloudformation.CreateChangeSetOutput{}, nil
			},
			mockWaitUntilChangeSetCreateComplete: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return nil
			},
			mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return &cloudformation.ExecuteChangeSetOutput{}, nil
			},
			mockWaitUntilStackUpdateComplete: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)

				return nil
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: mockCloudFormation{
					t: t,

					mockCreateStack:                      tc.mockCreateStack,
					mockWaitUntilStackCreateComplete:     tc.mockWaitUntilStackCreateComplete,
					mockCreateChangeSet:                  tc.mockCreateChangeSet,
					mockWaitUntilChangeSetCreateComplete: tc.mockWaitUntilChangeSetCreateComplete,
					mockExecuteChangeSet:                 tc.mockExecuteChangeSet,
					mockWaitUntilStackUpdateComplete:     tc.mockWaitUntilStackUpdateComplete,
				},
			}

			gotErr := cf.DeployApp(mockTemplate, mockStackName, mockChangeSetName)

			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
