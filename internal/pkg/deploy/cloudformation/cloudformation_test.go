// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

const (
	mockTemplate        = "mockTemplate"
	mockEnvironmentName = "mockEnvName"
	mockProjectName     = "mockProjectName"
	mockChangeSetID     = "mockChangeSetID"
	mockStackID         = "mockStackID"
)

type mockCloudFormation struct {
	cloudformationiface.CloudFormationAPI

	t                                               *testing.T
	mockCreateChangeSet                             func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
	mockWaitUntilChangeSetCreateCompleteWithContext func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
	mockExecuteChangeSet                            func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error)
	mockDescribeChangeSet                           func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error)
	mockWaitUntilStackCreateCompleteWithContext     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
	mockDescribeStacks                              func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	mockCreateStackSet                              func(t *testing.T, in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error)
	mockDescribeStackSet                            func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error)
	mockUpdateStackSet                              func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error)
	mockListStackInstances                          func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error)
	mockCreateStackInstances                        func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error)
	mockDescribeStackSetOperation                   func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error)
	mockDescribeStackEvents                         func(t *testing.T, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)
	mockCreateStack                                 func(t *testing.T, in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	mockWaitUntilStackUpdateCompleteWithContext     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
}

func (cf mockCloudFormation) CreateChangeSet(in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
	return cf.mockCreateChangeSet(cf.t, in)
}

func (cf mockCloudFormation) ExecuteChangeSet(in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
	return cf.mockExecuteChangeSet(cf.t, in)
}

func (cf mockCloudFormation) DescribeChangeSet(in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
	return cf.mockDescribeChangeSet(cf.t, in)
}

func (cf mockCloudFormation) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return cf.mockDescribeStacks(cf.t, in)
}

func (cf mockCloudFormation) CreateStackSet(in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error) {
	return cf.mockCreateStackSet(cf.t, in)
}

func (cf mockCloudFormation) DescribeStackSet(in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
	return cf.mockDescribeStackSet(cf.t, in)
}

func (cf mockCloudFormation) UpdateStackSet(in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
	return cf.mockUpdateStackSet(cf.t, in)
}

func (cf mockCloudFormation) ListStackInstances(in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
	return cf.mockListStackInstances(cf.t, in)
}

func (cf mockCloudFormation) CreateStackInstances(in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
	return cf.mockCreateStackInstances(cf.t, in)
}

func (cf mockCloudFormation) DescribeStackSetOperation(in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
	return cf.mockDescribeStackSetOperation(cf.t, in)
}

func (cf mockCloudFormation) DescribeStackEvents(in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
	return cf.mockDescribeStackEvents(cf.t, in)
}

func (cf mockCloudFormation) CreateStack(in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return cf.mockCreateStack(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilStackUpdateCompleteWithContext(context context.Context, in *cloudformation.DescribeStacksInput, opts ...request.WaiterOption) error {
	return cf.mockWaitUntilStackUpdateCompleteWithContext(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilChangeSetCreateCompleteWithContext(context context.Context, in *cloudformation.DescribeChangeSetInput, opts ...request.WaiterOption) error {
	return cf.mockWaitUntilChangeSetCreateCompleteWithContext(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilStackCreateCompleteWithContext(context context.Context, in *cloudformation.DescribeStacksInput, opts ...request.WaiterOption) error {
	return cf.mockWaitUntilStackCreateCompleteWithContext(cf.t, in)
}

type mockStackConfiguration struct {
	mockTemplate   func() (string, error)
	mockParameters func() []*cloudformation.Parameter
	mockTags       func() []*cloudformation.Tag
	mockStackName  func() string
}

func (sc mockStackConfiguration) Template() (string, error) {
	return sc.mockTemplate()
}

func (sc mockStackConfiguration) Parameters() []*cloudformation.Parameter {
	return sc.mockParameters()
}

func (sc mockStackConfiguration) Tags() []*cloudformation.Tag {
	return sc.mockTags()
}

func (sc mockStackConfiguration) StackName() string {
	return sc.mockStackName()
}

func TestDeploy(t *testing.T) {
	mockStackConfig := getMockStackConfiguration()
	testCases := map[string]struct {
		cf    CloudFormation
		input stackConfiguration
		want  error
	}{
		"should wrap error returned from CreateChangeSet call": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return nil, fmt.Errorf("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("failed to create changeSet for stack %s: %s", mockStackConfig.StackName(), "some AWS error"),
		},
		"should return a ErrStackAlreadyExists if the stack already exists": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						msg := fmt.Sprintf("Stack [%s-%s] already exists and cannot be created again with the changeSet [ecscli-%s]", mockProjectName, mockEnvironmentName, mockChangeSetID)
						return nil, awserr.New("ValidationError", msg, nil)
					},
				},
				box: boxWithTemplateFile(),
			},
			input: mockStackConfig,
			want: &ErrStackAlreadyExists{
				stackName: mockStackConfig.StackName(),
			},
		},
		"should wrap error returned from WaitUntilChangeSetCreateComplete call": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return errors.New("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("failed to wait for changeSet creation %s: %s", fmt.Sprintf("name=%s, stackID=%s", mockChangeSetID, mockStackID), "some AWS error"),
		},
		"should wrap error return from DescribeChangeSet call": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return nil, errors.New("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("failed to describe changeSet %s: %s", fmt.Sprintf("name=%s, stackID=%s", mockChangeSetID, mockStackID), "some AWS error"),
		},
		"should not execute Change Set with no changes": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusUnavailable),
							StatusReason:    aws.String(noChangesReason),
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			want: nil,
		},
		"should not execute Change Set with no updates": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusUnavailable),
							StatusReason:    aws.String(noUpdatesReason),
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			want: nil,
		},
		"should fail Change Set with unexpected status": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusUnavailable),
							StatusReason:    aws.String("some other reason"),
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			want: &ErrNotExecutableChangeSet{
				set: &changeSet{
					name:            mockChangeSetID,
					stackID:         mockStackID,
					executionStatus: cloudformation.ExecutionStatusUnavailable,
					statusReason:    "some other reason",
				},
			},
		},
		"should wrap error returned from ExecuteChangeSet call": {
			input: mockStackConfig,
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
						}, nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (output *cloudformation.ExecuteChangeSetOutput, e error) {
						return nil, errors.New("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("failed to execute changeSet %s: %s", fmt.Sprintf("name=%s, stackID=%s", mockChangeSetID, mockStackID), "some AWS error"),
		},
		"should deploy": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						require.Equal(t, mockStackConfig.StackName(), *in.StackName)
						require.True(t, isValidChangeSetName(*in.ChangeSetName))
						require.Equal(t, mockTemplate, *in.TemplateBody)

						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						require.Equal(t, mockStackID, *in.StackName)
						require.Equal(t, mockChangeSetID, *in.ChangeSetName)
						return nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
						}, nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (output *cloudformation.ExecuteChangeSetOutput, e error) {
						require.Equal(t, mockStackID, *in.StackName)
						require.Equal(t, mockChangeSetID, *in.ChangeSetName)
						return nil, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			input: mockStackConfig,
			want:  nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.deploy(tc.input, cloudformation.ChangeSetTypeCreate)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestWaitForStackCreation(t *testing.T) {
	stackConfig := getMockStackConfiguration()
	testCases := map[string]struct {
		cf    CloudFormation
		input stackConfiguration
		want  error
	}{
		"error in WaitUntilStackCreateComplete call": {
			cf:    getMockWaitStackCreateCFClient(t, stackConfig.StackName(), true, false),
			input: stackConfig,
			want:  fmt.Errorf("failed to create stack %s: %s", stackConfig.StackName(), "some AWS error"),
		},
		"error if no stacks returned": {
			cf:    getMockWaitStackCreateCFClient(t, stackConfig.StackName(), false, true),
			input: stackConfig,
			want:  fmt.Errorf("failed to find a stack named %s", stackConfig.StackName()),
		},
		"happy path": {
			cf:    getMockWaitStackCreateCFClient(t, stackConfig.StackName(), false, false),
			input: stackConfig,
			want:  nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, got := tc.cf.waitForStackCreation(tc.input)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func getMockWaitStackCreateCFClient(t *testing.T, stackName string, shouldThrowError, shouldReturnEmptyStacks bool) CloudFormation {
	return CloudFormation{
		client: &mockCloudFormation{
			t: t,
			mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, input *cloudformation.DescribeStacksInput) error {
				require.Equal(t, stackName, *input.StackName)
				if shouldThrowError {
					return fmt.Errorf("some AWS error")
				}
				return nil
			},
			mockDescribeStacks: func(t *testing.T, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
				require.Equal(t, stackName, *input.StackName)
				if shouldReturnEmptyStacks {
					return &cloudformation.DescribeStacksOutput{
						Stacks: []*cloudformation.Stack{},
					}, nil
				}
				return &cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackId: aws.String(fmt.Sprintf("arn:aws:cloudformation:eu-west-3:902697171733:stack/%s", stackName)),
						},
					},
				}, nil
			},
		},
		box: emptyBox(),
	}
}

func getMockStackConfiguration() stackConfiguration {
	return mockStackConfiguration{
		mockStackName: func() string {
			return mockStackID
		},
		mockParameters: func() []*cloudformation.Parameter {
			return []*cloudformation.Parameter{}
		},
		mockTags: func() []*cloudformation.Tag {
			return []*cloudformation.Tag{}
		},
		mockTemplate: func() (string, error) {
			return mockTemplate, nil
		},
	}
}

func emptyBox() packd.Box {
	return packd.NewMemoryBox()
}

func boxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(stack.EnvTemplatePath, mockTemplate)

	return box
}

// A change set name can contain only alphanumeric, case sensitive characters
// and hyphens. It must start with an alphabetic character and cannot exceed
// 128 characters.
func isValidChangeSetName(name string) bool {
	if len(name) > 128 {
		return false
	}
	matchesPattern := regexp.MustCompile(`[a-zA-Z][-a-zA-Z0-9]*`).MatchString
	if !matchesPattern(name) {
		return false
	}
	return true
}
