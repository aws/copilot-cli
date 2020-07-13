// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type stackDescriberMocks struct {
	mockStackDescriber *mocks.MockcfnStackDescriber
}

func TestStackDescriber_Stack(t *testing.T) {
	const mockStackName = "phonetool-test-jobs"
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks stackDescriberMocks)

		wantedStack *cloudformation.Stack
		wantedError error
	}{
		"return error if fail to describe stack": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
						StackName: aws.String(mockStackName),
					}).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("describe stack phonetool-test-jobs: some error"),
		},
		"return error if stack not found": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
						StackName: aws.String(mockStackName),
					}).Return(&cloudformation.DescribeStacksOutput{
						Stacks: []*cloudformation.Stack{},
					}, nil),
				)
			},
			wantedError: fmt.Errorf("stack phonetool-test-jobs not found"),
		},
		"success": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
						StackName: aws.String(mockStackName),
					}).Return(&cloudformation.DescribeStacksOutput{
						Stacks: []*cloudformation.Stack{
							{
								StackName: aws.String(mockStackName),
							},
						},
					}, nil),
				)
			},
			wantedStack: &cloudformation.Stack{
				StackName: aws.String(mockStackName),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStackDescriber := mocks.NewMockcfnStackDescriber(ctrl)
			mocks := stackDescriberMocks{
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &stackDescriber{
				stackDescribers: mockStackDescriber,
			}

			// WHEN
			actual, err := d.Stack(mockStackName)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedStack, actual)
			}
		})
	}
}

func TestStackDescriber_StackResources(t *testing.T) {
	const mockStackName = "phonetool-test-jobs"
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks stackDescriberMocks)

		wantedStackResources []*cloudformation.StackResource
		wantedError          error
	}{
		"return error if fail to describe stack resources": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(mockStackName),
					}).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("describe resources for stack phonetool-test-jobs: some error"),
		},
		"success": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(mockStackName),
					}).Return(&cloudformation.DescribeStackResourcesOutput{
						StackResources: []*cloudformation.StackResource{
							{
								StackName: aws.String(mockStackName),
							},
						},
					}, nil),
				)
			},
			wantedStackResources: []*cloudformation.StackResource{
				{
					StackName: aws.String(mockStackName),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStackDescriber := mocks.NewMockcfnStackDescriber(ctrl)
			mocks := stackDescriberMocks{
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &stackDescriber{
				stackDescribers: mockStackDescriber,
			}

			// WHEN
			actual, err := d.StackResources(mockStackName)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.wantedStackResources, actual)
			}
		})
	}
}
