// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy CLI concepts with AWS CloudFormation.
package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestStreamEnvironmentCreation(t *testing.T) {
	testCases := map[string]struct {
		in *deploy.CreateEnvironmentInput

		mockWaitUntilStackCreateCompleteWithContext func(t *testing.T, in *cloudformation.DescribeStacksInput) error
		mockDescribeStacks                          func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
		mockDescribeStackEvents                     func(t *testing.T, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)

		wantedEvents []deploy.ResourceEvent
		wantedResult deploy.CreateEnvironmentResponse
	}{
		"error while creating stack": {
			in: &deploy.CreateEnvironmentInput{
				Project: "phonetool",
				Name:    "test",
			},

			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return errors.New("wait until error")
			},
			mockDescribeStackEvents: func(t *testing.T, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return &cloudformation.DescribeStackEventsOutput{
					StackEvents: []*cloudformation.StackEvent{
						{
							LogicalResourceId:    aws.String("vpc"),
							ResourceType:         aws.String("AWS::EC2::VPC"),
							ResourceStatus:       aws.String(cloudformation.ResourceStatusCreateInProgress),
							ResourceStatusReason: aws.String("create initiated"),
						},
					},
				}, nil
			},

			wantedEvents: []deploy.ResourceEvent{
				{
					Resource: deploy.Resource{
						LogicalName: "vpc",
						Type:        "AWS::EC2::VPC",
					},
					Status:       cloudformation.ResourceStatusCreateInProgress,
					StatusReason: "create initiated",
				},
			},
			wantedResult: deploy.CreateEnvironmentResponse{
				Env: nil,
				Err: errors.New("failed to create stack phonetool-test: wait until error"),
			},
		},
		"swallows error while describing stack": {
			in: &deploy.CreateEnvironmentInput{
				Project: "phonetool",
				Name:    "test",
			},

			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return errors.New("wait until error")
			},
			mockDescribeStackEvents: func(t *testing.T, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return nil, errors.New("describe stack events error")
			},

			wantedEvents: nil,
			wantedResult: deploy.CreateEnvironmentResponse{
				Env: nil,
				Err: errors.New("failed to create stack phonetool-test: wait until error"),
			},
		},
		"sends an environment on success": {
			in: &deploy.CreateEnvironmentInput{
				Project: "phonetool",
				Name:    "test",
			},

			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return nil
			},
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackId: aws.String(fmt.Sprintf("arn:aws:cloudformation:eu-west-3:902697171733:stack/%s", "phonetool-test")),
						},
					},
				}, nil
			},
			mockDescribeStackEvents: func(t *testing.T, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
				require.Equal(t, "phonetool-test", *in.StackName, "stack names should be equal to each other")
				return &cloudformation.DescribeStackEventsOutput{
					StackEvents: []*cloudformation.StackEvent{
						{
							LogicalResourceId:    aws.String("vpc"),
							ResourceType:         aws.String("AWS::EC2::VPC"),
							ResourceStatus:       aws.String(cloudformation.ResourceStatusCreateInProgress),
							ResourceStatusReason: aws.String("create initiated"),
						},
					},
				}, nil
			},

			wantedEvents: []deploy.ResourceEvent{
				{
					Resource: deploy.Resource{
						LogicalName: "vpc",
						Type:        "AWS::EC2::VPC",
					},
					Status:       cloudformation.ResourceStatusCreateInProgress,
					StatusReason: "create initiated",
				},
			},
			wantedResult: deploy.CreateEnvironmentResponse{
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "eu-west-3",
					AccountID: "902697171733",
				},
				Err: nil,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			cf := CloudFormation{
				client: &mockCloudFormation{
					t:                       t,
					mockDescribeStackEvents: tc.mockDescribeStackEvents,
					mockDescribeStacks:      tc.mockDescribeStacks,
					mockWaitUntilStackCreateCompleteWithContext: tc.mockWaitUntilStackCreateCompleteWithContext,
				},
				box: emptyEnvBox(),
			}

			// WHEN
			events, resp := cf.StreamEnvironmentCreation(tc.in)

			// THEN
			require.Equal(t, tc.wantedEvents, <-events)
			got := <-resp
			if tc.wantedResult.Err != nil {
				require.EqualError(t, got.Err, tc.wantedResult.Err.Error(), "expected %v got %v", tc.wantedResult.Err, got.Err)
			} else {
				require.Equal(t, tc.wantedResult, got)
			}
		})
	}
}

func TestCloudFormation_DeleteEnvironment(t *testing.T) {
	const (
		testProject = "phonetool"
		testEnv     = "test"
	)
	testCases := map[string]struct {
		mockDescribeStacks                          func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
		mockDeleteStack                             func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
		mockWaitUntilStackDeleteCompleteWithContext func(t *testing.T, in *cloudformation.DescribeStacksInput) error

		wantedError error
	}{
		"stack does not exist": {
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("some error")
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return nil, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			wantedError: errors.New("some error"),
		},
		"delete stack fails": {
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackId: aws.String("arn:aws:cloudformation:us-west-1:1111:stack/phonetool-test"),
						},
					}}, nil
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return nil, errors.New("some error")
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			wantedError: errors.New("deleting stack arn:aws:cloudformation:us-west-1:1111:stack/phonetool-test: some error"),
		},
		"deletes stack successfully": {
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackId: aws.String("arn:aws:cloudformation:us-west-1:1111:stack/phonetool-test"),
						},
					}}, nil
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				require.Equal(t, "arn:aws:cloudformation:us-west-1:1111:stack/phonetool-test", *in.StackName)
				return nil, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: &mockCloudFormation{
					t:                  t,
					mockDescribeStacks: tc.mockDescribeStacks,
					mockDeleteStack:    tc.mockDeleteStack,
					mockWaitUntilStackDeleteCompleteWithContext: tc.mockWaitUntilStackDeleteCompleteWithContext,
				},
			}

			err := cf.DeleteEnvironment(testProject, testEnv)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func emptyEnvBox() packd.Box {
	return packd.NewMemoryBox()
}
