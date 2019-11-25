// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_UpdatePipeline(t *testing.T) {
	testCases := map[string]struct {
		in *deploy.CreatePipelineInput

		mockDescribeStacks                              func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
		mockCreateChangeSet                             func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
		mockWaitUntilChangeSetCreateCompleteWithContext func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
		mockDescribeChangeSet                           func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error)
		mockDeleteChangeSet                             func(t *testing.T, in *cloudformation.DeleteChangeSetInput) (*cloudformation.DeleteChangeSetOutput, error)

		wantedError error
	}{
		"deletes changeset and successfully exists if a stack update has no changes": {
			in: &deploy.CreatePipelineInput{
				ProjectName: "phonetool",
				Name:        "phonetool-pipeline",
				Source: &deploy.Source{
					ProviderName: manifest.GithubProviderName,
					Properties: map[string]interface{}{
						manifest.GithubSecretIdKeyName: "my secret",
						"repository":                   "github.com/hello/phonetool",
					},
				},
				Stages:          nil,
				ArtifactBuckets: nil,
			},

			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				// Stack already exists.
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				return &cloudformation.CreateChangeSetOutput{
					Id:      aws.String("1234"),
					StackId: aws.String("phonetool-1234"),
				}, nil
			},
			mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
				return errors.New("some changeset error")
			},
			mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
				return &cloudformation.DescribeChangeSetOutput{
					Changes:         []*cloudformation.Change{},
					ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					StatusReason:    aws.String(""),
				}, nil
			},
			mockDeleteChangeSet: func(t *testing.T, in *cloudformation.DeleteChangeSetInput) (*cloudformation.DeleteChangeSetOutput, error) {
				require.Equal(t, "1234", *in.ChangeSetName)
				require.Equal(t, "phonetool-1234", *in.StackName)
				return &cloudformation.DeleteChangeSetOutput{}, nil
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			cf := CloudFormation{
				client: &mockCloudFormation{
					t:                   t,
					mockDescribeStacks:  tc.mockDescribeStacks,
					mockCreateChangeSet: tc.mockCreateChangeSet,
					mockWaitUntilChangeSetCreateCompleteWithContext: tc.mockWaitUntilChangeSetCreateCompleteWithContext,
					mockDescribeChangeSet:                           tc.mockDescribeChangeSet,
					mockDeleteChangeSet:                             tc.mockDeleteChangeSet,
				},
				box: boxWithTemplateFile(),
			}

			// WHEN
			err := cf.UpdatePipeline(tc.in)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestCloudFormation_CreatePipeline(t *testing.T) {
	input := &deploy.CreatePipelineInput{
		ProjectName: "phonetool",
		Name:        "phonetool-pipeline",
		Source: &deploy.Source{
			ProviderName: manifest.GithubProviderName,
			Properties: map[string]interface{}{
				manifest.GithubSecretIdKeyName: "my secret",
				"repository":                   "github.com/hello/phonetool",
			},
		},
		Stages:          nil,
		ArtifactBuckets: nil,
	}
	testCases := map[string]struct {
		in *deploy.CreatePipelineInput

		mockCreateChangeSet                             func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
		mockDescribeStacks                              func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
		mockWaitUntilChangeSetCreateCompleteWithContext func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
		mockDescribeChangeSet                           func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error)
		mockExecuteChangeSet                            func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error)
		mockWaitUntilStackCreateCompleteWithContext     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
		wantedError                                     error
	}{
		"successfully create a pipeline if a stack does not exist": {
			in: input,
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				// Stack does not exist.
				pipelineConfig := stack.NewPipelineStackConfig(input)
				return nil, &ErrStackNotFound{stackName: pipelineConfig.StackName()}
			},
			mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
				return &cloudformation.CreateChangeSetOutput{
					Id:      aws.String("1234"),
					StackId: aws.String("phonetool-1234"),
				}, nil
			},
			mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
				return &cloudformation.DescribeChangeSetOutput{
					Changes: []*cloudformation.Change{
						&cloudformation.Change{},
					},
					ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					StatusReason:    aws.String(""),
				}, nil
			},
			mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
				return nil
			},
			mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
				return &cloudformation.ExecuteChangeSetOutput{}, nil
			},
			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			wantedError: &ErrStackNotFound{stackName: "phonetool-phonetool-pipeline"},
		},
		"return stack exists error if a stack exists": {
			in: input,
			mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				// Stack already exists.
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil
			},
			wantedError: errors.New("stack phonetool-phonetool-pipeline already exists"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			cf := CloudFormation{
				client: &mockCloudFormation{
					t:                   t,
					mockDescribeStacks:  tc.mockDescribeStacks,
					mockCreateChangeSet: tc.mockCreateChangeSet,
					mockWaitUntilChangeSetCreateCompleteWithContext: tc.mockWaitUntilChangeSetCreateCompleteWithContext,
					mockDescribeChangeSet:                           tc.mockDescribeChangeSet,
					mockExecuteChangeSet:                            tc.mockExecuteChangeSet,
					mockWaitUntilStackCreateCompleteWithContext:     tc.mockWaitUntilStackCreateCompleteWithContext,
				},
				box: boxWithTemplateFile(),
			}

			// WHEN
			err := cf.CreatePipeline(tc.in)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
