// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestDeleteStack(t *testing.T) {
	mockStackName := "mockStackName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		mockDeleteStack func(*testing.T, *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)

		want error
	}{
		"should wrap error returned by DeleteStack": {
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return nil, mockError
			},
			want: fmt.Errorf("delete stack %s: %w", mockStackName, mockError),
		},
		"should return nil given success": {
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)

				return &cloudformation.DeleteStackOutput{}, nil
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: mockCloudFormation{
					t: t,

					mockDeleteStack: test.mockDeleteStack,
				},
			}

			got := cf.DeleteStack(mockStackName)

			require.Equal(t, test.want, got)
		})
	}
}

func TestWaitForStackDelete(t *testing.T) {
	mockStackName := "mockStackName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		mockWaitUntilStackDeleteComplete func(t *testing.T, in *cloudformation.DescribeStacksInput) error

		want error
	}{
		"should wrap error returned by DeleteStack": {
			mockWaitUntilStackDeleteComplete: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return mockError
			},
			want: fmt.Errorf("wait until stack delete complete %s: %w", mockStackName, mockError),
		},
		"should return nil given success": {
			mockWaitUntilStackDeleteComplete: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				t.Helper()

				require.Equal(t, mockStackName, *in.StackName)

				return nil
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: mockCloudFormation{
					t: t,

					mockWaitUntilStackDeleteComplete: test.mockWaitUntilStackDeleteComplete,
				},
			}

			got := cf.WaitForStackDelete(mockStackName)

			require.Equal(t, test.want, got)
		})
	}
}
