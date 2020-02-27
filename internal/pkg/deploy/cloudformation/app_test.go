// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestDeployApp(t *testing.T) {
	mockTemplate := "mockTemplate"
	mockStackName := "mockStackName"
	mockChangeSetName := "mockChangeSetName"
	mockExecutionRole := "mockExecutionRole"
	mockError := errors.New("mockError")

	testCases := map[string]struct {
		mockCreateStack                                 func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
		mockWaitUntilStackCreateCompleteWithContext     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
		mockCreateChangeSet                             func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
		mockWaitUntilChangeSetCreateCompleteWithContext func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
		mockExecuteChangeSet                            func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error)
		mockWaitUntilStackUpdateCompleteWithContext     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
		mockDescribeChangeSet                           func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error)

		wantErr error
	}{
		"should create the stack if one didn't exist already and wait for completion": {
			mockCreateStack: func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return &cloudformation.CreateStackOutput{}, nil
			},
			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
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
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil)
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, cloudformation.ChangeSetTypeUpdate, *in.ChangeSetType)
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return &cloudformation.CreateChangeSetOutput{}, nil
			},
			mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
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
			mockWaitUntilStackUpdateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)

				return nil
			},
		},
		"should describe stack to check for no changes scenario if WaitUntilChangeSetCreateComplete fails": {
			mockCreateStack: func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil)
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, cloudformation.ChangeSetTypeUpdate, *in.ChangeSetType)
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return &cloudformation.CreateChangeSetOutput{}, nil
			},
			mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return mockError
			},
			mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return &cloudformation.DescribeChangeSetOutput{
					Changes: []*cloudformation.Change{},
				}, nil
			},
		},
		"should wrap DescribeChangeSet error if WaitUntilChangeSetCreateComplete fails": {
			mockCreateStack: func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil)
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)
				require.Equal(t, mockTemplate, *in.TemplateBody)
				require.Equal(t, cloudformation.CapabilityCapabilityIam, *in.Capabilities[0])
				require.Equal(t, cloudformation.ChangeSetTypeUpdate, *in.ChangeSetType)
				require.Equal(t, mockExecutionRole, *in.RoleARN)

				return &cloudformation.CreateChangeSetOutput{}, nil
			},
			mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return mockError
			},
			mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
				t.Helper()

				require.Equal(t, mockChangeSetName, *in.ChangeSetName)
				require.Equal(t, mockStackName, *in.StackName)

				return nil, mockError
			},
			wantErr: fmt.Errorf("describe change set: %w", mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: mockCloudFormation{
					t: t,

					mockCreateStack: tc.mockCreateStack,
					mockWaitUntilStackCreateCompleteWithContext:     tc.mockWaitUntilStackCreateCompleteWithContext,
					mockCreateChangeSet:                             tc.mockCreateChangeSet,
					mockWaitUntilChangeSetCreateCompleteWithContext: tc.mockWaitUntilChangeSetCreateCompleteWithContext,
					mockExecuteChangeSet:                            tc.mockExecuteChangeSet,
					mockWaitUntilStackUpdateCompleteWithContext:     tc.mockWaitUntilStackUpdateCompleteWithContext,
					mockDescribeChangeSet:                           tc.mockDescribeChangeSet,
				},
			}

			gotErr := cf.DeployApp(mockTemplate, mockStackName, mockChangeSetName, mockExecutionRole, nil, nil)

			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
