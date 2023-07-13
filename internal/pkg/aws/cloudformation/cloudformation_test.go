// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	mockChangeSetName = "copilot-31323334-3536-4738-b930-313233333435"
	mockChangeSetID   = "arn:aws:cloudformation:us-west-2:111:changeSet/copilot-31323334-3536-4738-b930-313233333435/9edc39b0-ee18-440d-823e-3dda74646b2"
)

var (
	mockStack = NewStack("id", "template")

	errDoesNotExist             = awserr.New("ValidationError", "does not exist", nil)
	errStackNotInUpdateProgress = awserr.New("ValidationError", "CancelUpdateStack cannot be called from current stack status", nil)
)

func TestCloudFormation_Create(t *testing.T) {
	testCases := map[string]struct {
		inStack    *Stack
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"fail if checking the stack description fails": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.Name),
				}).Return(nil, errors.New("some unexpected error"))
				return m
			},
			wantedErr: fmt.Errorf("describe stack %s: %w", mockStack.Name, errors.New("some unexpected error")),
		},
		"fail if a stack exists that's already in progress": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateInProgress),
						},
					},
				}, nil)
				return m
			},
			wantedErr: &ErrStackUpdateInProgress{
				Name: mockStack.Name,
			},
		},
		"fail if a successfully created stack already exists": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil)
				return m
			},
			wantedErr: &ErrStackAlreadyExists{
				Name: mockStack.Name,
				Stack: &StackDescription{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
		},
		"creates the stack if it doesn't exist": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				addCreateDeployCalls(m)
				return m
			},
		},
		"creates the stack with automatic stack rollback disabled": {
			inStack: NewStack("id", "template", WithDisableRollback()),
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStack.Name),
					ChangeSetType:       aws.String(cloudformation.ChangeSetTypeCreate),
					TemplateBody:        aws.String(mockStack.TemplateBody),
					Parameters:          nil,
					Tags:                nil,
					RoleARN:             nil,
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id:      aws.String(mockChangeSetID),
					StackId: aws.String(mockStack.Name),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetID),
				}, gomock.Any())
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetID),
					StackName:     aws.String(mockStack.Name),
				}).Return(&cloudformation.DescribeChangeSetOutput{
					Changes: []*cloudformation.Change{
						{
							ResourceChange: &cloudformation.ResourceChange{
								ResourceType: aws.String("ecs service"),
							},
							Type: aws.String(cloudformation.ChangeTypeResource),
						},
					},
					ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					StatusReason:    aws.String("some reason"),
				}, nil)
				m.EXPECT().ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
					ChangeSetName:   aws.String(mockChangeSetID),
					StackName:       aws.String(mockStack.Name),
					DisableRollback: aws.Bool(true),
				})
				return m
			},
		},
		"creates the stack with templateURL": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				addCreateDeployCalls(m)
				return m
			},
		},
		"creates the stack after cleaning the previously failed execution": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusRollbackComplete),
						},
					},
				}, nil)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.Name),
				})
				m.EXPECT().WaitUntilStackDeleteCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.Name),
				}, gomock.Any(), gomock.Any())
				addCreateDeployCalls(m)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			seed := bytes.NewBufferString("12345678901233456789") // always generate the same UUID
			uuid.SetRand(seed)
			defer uuid.SetRand(nil)

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			id, err := c.Create(tc.inStack)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, mockChangeSetID, id)
			}
		})
	}
}

func TestCloudFormation_DescribeChangeSet(t *testing.T) {
	t.Run("returns an error if the DescribeChangeSet action fails", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMockclient(ctrl)
		m.EXPECT().DescribeChangeSet(gomock.Any()).Return(nil, errors.New("some error"))
		cfn := CloudFormation{
			client: m,
		}

		// WHEN
		out, err := cfn.DescribeChangeSet(mockChangeSetID, "phonetool-test")

		// THEN
		require.EqualError(t, err, fmt.Sprintf("describe change set %s for stack phonetool-test: some error", mockChangeSetID))
		require.Nil(t, out)
	})

	t.Run("calls DescribeChangeSet repeatedly if there is a next token", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMockclient(ctrl)
		wantedChanges := []*cloudformation.Change{
			{
				ResourceChange: &cloudformation.ResourceChange{
					ResourceType: aws.String("AWS::ECS::Service"),
				},
			},
			{
				ResourceChange: &cloudformation.ResourceChange{
					ResourceType: aws.String("AWS::ECS::Cluster"),
				},
			},
		}
		gomock.InOrder(
			m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
				ChangeSetName: aws.String(mockChangeSetID),
				StackName:     aws.String("phonetool-test"),
				NextToken:     nil,
			}).Return(&cloudformation.DescribeChangeSetOutput{
				Changes: []*cloudformation.Change{
					wantedChanges[0],
				},
				NextToken: aws.String("1111"),
			}, nil),
			m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
				ChangeSetName: aws.String(mockChangeSetID),
				StackName:     aws.String("phonetool-test"),
				NextToken:     aws.String("1111"),
			}).Return(&cloudformation.DescribeChangeSetOutput{
				Changes: []*cloudformation.Change{
					wantedChanges[1],
				},
			}, nil),
		)

		cfn := CloudFormation{
			client: m,
		}

		// WHEN
		out, err := cfn.DescribeChangeSet(mockChangeSetID, "phonetool-test")

		// THEN
		require.NoError(t, err)
		require.Equal(t, wantedChanges, out.Changes)
	})
}

func TestCloudFormation_WaitForCreate(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"wraps error on failure": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().WaitUntilStackCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.Name),
				}, gomock.Any()).Return(errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("wait until stack %s create is complete: %w", mockStack.Name, errors.New("some error")),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			err := c.WaitForCreate(context.Background(), mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_Update(t *testing.T) {
	const (
		mockStackName     = "id"
		mockChangeSetName = "copilot-31323334-3536-4738-b930-313233333435"
	)
	testCases := map[string]struct {
		inStack    *Stack
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"fail if the stack is already in progress": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateInProgress)}},
				}, nil)
				return m
			},
			wantedErr: &ErrStackUpdateInProgress{
				Name: mockStack.Name,
			},
		},
		"error if fail to create the changeset because of random issue": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						Changes:      []*cloudformation.Change{},
						StatusReason: aws.String("some other reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("create change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error: some other reason"),
		},
		"error if fail to wait until the changeset creation complete": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						StatusReason: aws.String("some reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("wait for creation of change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error: some reason"),
		},
		"error if fail to describe change set after creation failed": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						NextToken: aws.String("mockNext"),
					}, nil)
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName),
					NextToken:     aws.String("mockNext")}).
					Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("check if changeset is empty: create change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error: describe change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error"),
		},
		"delete change set and throw ErrChangeSetEmpty if failed to create the change set because it is empty": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						Changes:      []*cloudformation.Change{},
						StatusReason: aws.String("The submitted information didn't contain changes. Submit different information to create a change set."),
					}, nil)
				m.EXPECT().DeleteChangeSet(&cloudformation.DeleteChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName),
				}).Return(nil, nil)
				return m
			},
			wantedErr: fmt.Errorf("change set with name copilot-31323334-3536-4738-b930-313233333435 for stack id has no changes"),
		},
		"error if creation succeed but failed to describe change set": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("describe change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error"),
		},
		"ignore execute request if the change set does not contain any modifications": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("UPDATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("template"),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(&cloudformation.DescribeChangeSetOutput{
						ExecutionStatus: aws.String(cloudformation.ExecutionStatusUnavailable),
						StatusReason:    aws.String(noChangesReason),
					}, nil)
				return m
			},
		},
		"error if change set is not executable": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						ExecutionStatus: aws.String(cloudformation.ExecutionStatusUnavailable),
						StatusReason:    aws.String("some other reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("execute change set copilot-31323334-3536-4738-b930-313233333435 for stack id because status is UNAVAILABLE with reason some other reason"),
		},
		"error if fail to execute change set": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(gomock.Any()).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(gomock.Any()).
					Return(&cloudformation.DescribeChangeSetOutput{
						ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					}, nil)
				m.EXPECT().ExecuteChangeSet(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("execute change set copilot-31323334-3536-4738-b930-313233333435 for stack id: some error"),
		},
		"updates the stack with automatic stack rollback disabled": {
			inStack: NewStack("id", "template", WithDisableRollback()),
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("UPDATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("template"),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(&cloudformation.DescribeChangeSetOutput{
						ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					}, nil)
				m.EXPECT().ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
					ChangeSetName:   aws.String(mockChangeSetName),
					StackName:       aws.String(mockStackName),
					DisableRollback: aws.Bool(true),
				}).Return(&cloudformation.ExecuteChangeSetOutput{}, nil)
				return m
			},
		},
		"success": {
			inStack: mockStack,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{{StackStatus: aws.String(cloudformation.StackStatusUpdateComplete)}},
				}, nil)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("UPDATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("template"),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(&cloudformation.DescribeChangeSetOutput{
						ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
					}, nil)
				m.EXPECT().ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName),
				}).Return(&cloudformation.ExecuteChangeSetOutput{}, nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			seed := bytes.NewBufferString("12345678901233456789") // always generate the same UUID
			uuid.SetRand(seed)
			defer uuid.SetRand(nil)

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			id, err := c.Update(tc.inStack)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, mockChangeSetName, id)
			}
		})
	}
}

func TestCloudFormation_UpdateAndWait(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"waits until the stack is created": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil)
				addUpdateDeployCalls(m)
				m.EXPECT().WaitUntilStackUpdateCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.Name),
				}, gomock.Any()).Return(nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			seed := bytes.NewBufferString("12345678901233456789") // always generate the same UUID
			uuid.SetRand(seed)
			defer uuid.SetRand(nil)

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			err := c.UpdateAndWait(mockStack)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_Delete(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"fails on unexpected error": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DeleteStack(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("delete stack %s: %w", mockStack.Name, errors.New("some error")),
		},
		"exits successfully if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.Name),
				}).Return(nil, errDoesNotExist)
				return m
			},
		},
		"exits successfully if stack can be deleted": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.Name),
				}).Return(nil, nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			err := c.Delete(mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_DeleteAndWait(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedErr  error
	}{
		"skip waiting if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DeleteStack(gomock.Any()).Return(nil, errDoesNotExist)
				m.EXPECT().WaitUntilStackDeleteCompleteWithContext(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				return m
			},
		},
		"wait for stack deletion if stack is being deleted": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.Name),
				}).Return(nil, nil)
				m.EXPECT().WaitUntilStackDeleteCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.Name),
				}, gomock.Any())
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			err := c.DeleteAndWait(mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestStackDescriber_Metadata(t *testing.T) {
	testCases := map[string]struct {
		isStackSet bool
		createMock func(ctrl *gomock.Controller) client

		wantedMetadata string
		wantedErr      error
	}{
		"should wrap cfn error on unexpected error": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplateSummary(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},

			wantedErr: errors.New("get template summary: some error"),
		},
		"should return ErrStackNotFound if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplateSummary(gomock.Any()).Return(nil, errDoesNotExist)
				return m
			},

			wantedErr: errors.New("stack named phonetoolStack cannot be found"),
		},
		"should return ErrStackNotFound if stackset does not exist": {
			isStackSet: true,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplateSummary(gomock.Any()).Return(nil, errDoesNotExist)
				return m
			},

			wantedErr: errors.New("stack named phonetoolStackSet cannot be found"),
		},
		"should return Metadata property of template summary on success for stack": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplateSummary(&cloudformation.GetTemplateSummaryInput{
					StackName: aws.String("phonetoolStack"),
				}).Return(&cloudformation.GetTemplateSummaryOutput{
					Metadata: aws.String("hello"),
				}, nil)
				return m
			},

			wantedMetadata: "hello",
		},
		"should return Metadata property of template summary on success for stack set": {
			isStackSet: true,
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplateSummary(&cloudformation.GetTemplateSummaryInput{
					StackSetName: aws.String("phonetoolStackSet"),
				}).Return(&cloudformation.GetTemplateSummaryOutput{
					Metadata: aws.String("hello"),
				}, nil)
				return m
			},

			wantedMetadata: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			name := MetadataWithStackName("phonetoolStack")
			if tc.isStackSet {
				name = MetadataWithStackSetName("phonetoolStackSet")
			}
			actual, err := c.Metadata(name)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedMetadata, actual)
			}
		})
	}
}

func TestCloudFormation_Describe(t *testing.T) {
	testCases := map[string]struct {
		createMock  func(ctrl *gomock.Controller) client
		wantedDescr *StackDescription
		wantedErr   error
	}{
		"return ErrStackNotFound if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				return m
			},
			wantedErr: &ErrStackNotFound{name: mockStack.Name},
		},
		"returns ErrStackNotFound if the list returned is empty": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{},
				}, nil)
				return m
			},
			wantedErr: &ErrStackNotFound{name: mockStack.Name},
		},
		"returns a StackDescription if stack exists": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName: aws.String(mockStack.Name),
						},
					},
				}, nil)
				return m
			},
			wantedDescr: &StackDescription{
				StackName: aws.String(mockStack.Name),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			descr, err := c.Describe(mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedDescr, descr)
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_Exists(t *testing.T) {
	t.Run("should return underlying error on unexpected describe error", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		wantedErr := errors.New("some error")

		m := mocks.NewMockclient(ctrl)
		m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, wantedErr)
		c := CloudFormation{
			client: m,
		}

		// WHEN
		_, err := c.Exists("phonetool-test")

		// THEN
		require.EqualError(t, err, "describe stack phonetool-test: some error")
	})
	t.Run("should return false if the stack is not found", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMockclient(ctrl)
		m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
		c := CloudFormation{
			client: m,
		}

		// WHEN
		exists, err := c.Exists("phonetool-test")

		// THEN
		require.NoError(t, err)
		require.False(t, exists)
	})
	t.Run("should return true if the stack is found", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMockclient(ctrl)
		m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String("phonetool-test"),
		}).Return(&cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{{}},
		}, nil)
		c := CloudFormation{
			client: m,
		}

		// WHEN
		exists, err := c.Exists("phonetool-test")

		// THEN
		require.NoError(t, err)
		require.True(t, exists)
	})
}

func TestCloudFormation_TemplateBody(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedBody string
		wantedErr  error
	}{
		"return ErrStackNotFound if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplate(gomock.Any()).Return(nil, errDoesNotExist)
				return m
			},
			wantedErr: &ErrStackNotFound{name: mockStack.Name},
		},
		"returns the template body if the stack exists": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplate(&cloudformation.GetTemplateInput{
					StackName: aws.String(mockStack.Name),
				}).Return(&cloudformation.GetTemplateOutput{
					TemplateBody: aws.String("hello"),
				}, nil)
				return m
			},
			wantedBody: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			body, err := c.TemplateBody(mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedBody, body)
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_TemplateBodyFromChangeSet(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client
		wantedBody string
		wantedErr  string
	}{
		"return ErrStackNotFound if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplate(gomock.Any()).Return(nil, errDoesNotExist)
				return m
			},
			wantedErr: (&ErrStackNotFound{name: mockStack.Name}).Error(),
		},
		"returns wrapped error on expected error": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplate(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Sprintf("get template for stack %s and change set %s: some error", mockStack.Name, mockChangeSetID),
		},
		"returns the template body if the change set and stack exists": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().GetTemplate(&cloudformation.GetTemplateInput{
					ChangeSetName: aws.String(mockChangeSetID),
					StackName:     aws.String(mockStack.Name),
				}).Return(&cloudformation.GetTemplateOutput{
					TemplateBody: aws.String("hello"),
				}, nil)
				return m
			},
			wantedBody: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			body, err := c.TemplateBodyFromChangeSet(mockChangeSetID, mockStack.Name)

			// THEN
			if tc.wantedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantedBody, body)
			} else {
				require.EqualError(t, err, tc.wantedErr)
			}
		})
	}
}

func TestCloudFormation_Outputs(t *testing.T) {
	testCases := map[string]struct {
		createMock    func(ctrl *gomock.Controller) client
		wantedOutputs map[string]string
		wantedErr     string
	}{
		"successfully returns outputs": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName: aws.String(mockStack.Name),
							Outputs: []*cloudformation.Output{
								{
									OutputKey:   aws.String("PipelineConnection"),
									OutputValue: aws.String("mockARN"),
								},
							},
						},
					},
				}, nil)
				return m
			},
			wantedOutputs: map[string]string{
				"PipelineConnection": "mockARN",
			},
		},
		"wraps error from Describe()": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, fmt.Errorf("some error"))
				return m
			},
			wantedOutputs: nil,
			wantedErr:     "retrieve outputs of stack description: describe stack id: some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			outputs, err := c.Outputs(mockStack)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, tc.wantedOutputs, outputs)
			}
		})
	}
}

func TestCloudFormation_ErrorEvents(t *testing.T) {
	mockEvents := []*cloudformation.StackEvent{
		{
			LogicalResourceId:    aws.String("abc123"),
			ResourceType:         aws.String("ECS::Service"),
			ResourceStatus:       aws.String("CREATE_FAILED"),
			ResourceStatusReason: aws.String("Space elevator disconnected. (Service moonshot)"),
		},
		{
			LogicalResourceId:    aws.String("xyz"),
			ResourceType:         aws.String("ECS::Service"),
			ResourceStatus:       aws.String("CREATE_COMPLETE"),
			ResourceStatusReason: aws.String("Moon landing achieved. (Service moonshot)"),
		},
	}
	testCases := map[string]struct {
		mockCf       func(mockclient *mocks.Mockclient)
		wantedErr    string
		wantedEvents []StackEvent
	}{
		"completes successfully": {
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
					StackName: aws.String(mockStack.Name),
				}).Return(&cloudformation.DescribeStackEventsOutput{
					StackEvents: mockEvents,
				}, nil)
			},
			wantedEvents: []StackEvent{
				{
					LogicalResourceId:    aws.String("abc123"),
					ResourceType:         aws.String("ECS::Service"),
					ResourceStatus:       aws.String("CREATE_FAILED"),
					ResourceStatusReason: aws.String("Space elevator disconnected. (Service moonshot)"),
				},
			},
		},
		"error retrieving events": {
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: "describe stack events for stack id: some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCf := mocks.NewMockclient(ctrl)
			tc.mockCf(mockCf)

			c := CloudFormation{
				client: mockCf,
			}
			// WHEN
			events, err := c.ErrorEvents(mockStack.Name)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, tc.wantedEvents, events)
			}
		})
	}
}

func TestCloudFormation_Events(t *testing.T) {
	testCases := map[string]struct {
		createMock   func(ctrl *gomock.Controller) client
		wantedEvents []StackEvent
		wantedErr    error
	}{
		"return events in chronological order": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
					StackName: aws.String(mockStack.Name),
				}).Return(&cloudformation.DescribeStackEventsOutput{
					StackEvents: []*cloudformation.StackEvent{
						{
							ResourceType: aws.String("ecs"),
						},
						{
							ResourceType: aws.String("s3"),
						},
					},
				}, nil)
				return m
			},
			wantedEvents: []StackEvent{
				{
					ResourceType: aws.String("s3"),
				},
				{
					ResourceType: aws.String("ecs"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			events, err := c.Events(mockStack.Name)

			// THEN
			require.Equal(t, tc.wantedEvents, events)
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestStackDescriber_StackResources(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client

		wantedStackResources []*StackResource
		wantedError          error
	}{
		"return a wrapped error if fail to describe stack resources": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStackResources(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedError: fmt.Errorf("describe resources for stack phonetool-test-api: some error"),
		},
		"returns type-casted stack resources on success": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
					StackName: aws.String("phonetool-test-api"),
				}).Return(&cloudformation.DescribeStackResourcesOutput{
					StackResources: []*cloudformation.StackResource{
						{
							StackName: aws.String("phonetool-test-api"),
						},
					},
				}, nil)
				return m
			},
			wantedStackResources: []*StackResource{
				{
					StackName: aws.String("phonetool-test-api"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			actual, err := c.StackResources("phonetool-test-api")

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedStackResources, actual)
			}
		})
	}
}

func TestCloudFormation_ListStacksWithTags(t *testing.T) {
	mockAppTag := cloudformation.Tag{
		Key:   aws.String("copilot-application"),
		Value: aws.String("phonetool"),
	}
	mockEnvTag := cloudformation.Tag{
		Key:   aws.String("copilot-environment"),
		Value: aws.String("test-pdx"),
	}
	mockTaskTag1 := cloudformation.Tag{
		Key:   aws.String("copilot-task"),
		Value: aws.String("db-migrate"),
	}
	mockTaskTag2 := cloudformation.Tag{
		Key:   aws.String("copilot-task"),
		Value: aws.String("default-oneoff"),
	}
	mockStack1 := cloudformation.Stack{
		StackName: aws.String("task-appenv"),
		Tags: []*cloudformation.Tag{
			&mockAppTag,
			&mockEnvTag,
			&mockTaskTag1,
		},
	}
	mockStack2 := cloudformation.Stack{
		StackName: aws.String("task-default-oneoff"),
		Tags: []*cloudformation.Tag{
			&mockTaskTag2,
		},
	}
	mockStack3 := cloudformation.Stack{
		StackName: aws.String("phonetool-test-pdx"),
		Tags: []*cloudformation.Tag{
			&mockAppTag,
			&mockEnvTag,
		},
	}
	mockStacks := &cloudformation.DescribeStacksOutput{
		Stacks: []*cloudformation.Stack{
			&mockStack1,
			&mockStack2,
			&mockStack3,
		},
	}
	testCases := map[string]struct {
		inTags       map[string]string
		mockCf       func(*mocks.Mockclient)
		wantedStacks []StackDescription
		wantedErr    string
	}{
		"successfully lists stacks with tags": {
			inTags: map[string]string{
				"copilot-application": "phonetool",
				"copilot-environment": "test-pdx",
			},
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{}).Return(mockStacks, nil)
			},
			wantedStacks: []StackDescription{
				{
					StackName: aws.String("task-appenv"),
					Tags: []*cloudformation.Tag{
						&mockAppTag,
						&mockEnvTag,
						&mockTaskTag1,
					},
				},
				{
					StackName: aws.String("phonetool-test-pdx"),
					Tags: []*cloudformation.Tag{
						&mockAppTag,
						&mockEnvTag,
					},
				},
			},
		},
		"lists all stacks with wildcard tag": {
			inTags: map[string]string{
				"copilot-task": "",
			},
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{}).Return(&cloudformation.DescribeStacksOutput{
					NextToken: aws.String("abc"),
					Stacks: []*cloudformation.Stack{
						&mockStack1,
					},
				}, nil)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					NextToken: aws.String("abc"),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						&mockStack2,
						&mockStack3,
					},
				}, nil)
			},
			wantedStacks: []StackDescription{
				{
					StackName: aws.String("task-appenv"),
					Tags: []*cloudformation.Tag{
						&mockAppTag,
						&mockEnvTag,
						&mockTaskTag1,
					},
				},
				{
					StackName: aws.String("task-default-oneoff"),
					Tags: []*cloudformation.Tag{
						&mockTaskTag2,
					},
				},
			},
		},
		"empty map returns all stacks": {
			inTags: map[string]string{},
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{}).Return(mockStacks, nil)
			},
			wantedStacks: []StackDescription{
				{
					StackName: aws.String("task-appenv"),
					Tags: []*cloudformation.Tag{
						&mockAppTag,
						&mockEnvTag,
						&mockTaskTag1,
					},
				},
				{
					StackName: aws.String("task-default-oneoff"),
					Tags: []*cloudformation.Tag{
						&mockTaskTag2,
					},
				},
				{
					StackName: aws.String("phonetool-test-pdx"),
					Tags: []*cloudformation.Tag{
						&mockAppTag,
						&mockEnvTag,
					},
				},
			},
		},
		"error listing stacks": {
			inTags: map[string]string{},
			mockCf: func(m *mocks.Mockclient) {
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{}).Return(nil, errors.New("some error"))
			},
			wantedErr: "list stacks: some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockclient(ctrl)
			tc.mockCf(mockClient)

			c := CloudFormation{
				client: mockClient,
			}

			// WHEN
			stacks, err := c.ListStacksWithTags(tc.inTags)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, tc.wantedStacks, stacks)
			}
		})
	}
}

func TestCloudformation_CancelUpdateStack(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) client

		wantedErr error
	}{
		"return a wrapped error if fail to cancel stack update": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().CancelUpdateStack(&cloudformation.CancelUpdateStackInput{
					StackName: aws.String("phonetool-test-api"),
				}).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("cancel update stack: some error"),
		},
		"return nil if the stack is not found": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().CancelUpdateStack(&cloudformation.CancelUpdateStackInput{
					StackName: aws.String("phonetool-test-api"),
				}).Return(nil, errDoesNotExist)
				return m
			},
		},
		"return nil if the stack is not in update progress state": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().CancelUpdateStack(&cloudformation.CancelUpdateStackInput{
					StackName: aws.String("phonetool-test-api"),
				}).Return(nil, errStackNotInUpdateProgress)
				return m
			},
		},
		"success": {
			createMock: func(ctrl *gomock.Controller) client {
				m := mocks.NewMockclient(ctrl)
				m.EXPECT().CancelUpdateStack(&cloudformation.CancelUpdateStackInput{
					StackName: aws.String("phonetool-test-api"),
				}).Return(&cloudformation.CancelUpdateStackOutput{}, nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				client: tc.createMock(ctrl),
			}

			// WHEN
			err := c.CancelUpdateStack("phonetool-test-api")

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func addCreateDeployCalls(m *mocks.Mockclient) {
	addDeployCalls(m, cloudformation.ChangeSetTypeCreate)
}

func addUpdateDeployCalls(m *mocks.Mockclient) {
	addDeployCalls(m, cloudformation.ChangeSetTypeUpdate)
}

func addDeployCalls(m *mocks.Mockclient, changeSetType string) {
	m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
		ChangeSetName:       aws.String(mockChangeSetName),
		StackName:           aws.String(mockStack.Name),
		ChangeSetType:       aws.String(changeSetType),
		TemplateBody:        aws.String(mockStack.TemplateBody),
		Parameters:          nil,
		Tags:                nil,
		RoleARN:             nil,
		IncludeNestedStacks: aws.Bool(true),
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
			cloudformation.CapabilityCapabilityNamedIam,
			cloudformation.CapabilityCapabilityAutoExpand,
		}),
	}).Return(&cloudformation.CreateChangeSetOutput{
		Id:      aws.String(mockChangeSetID),
		StackId: aws.String(mockStack.Name),
	}, nil)
	m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetID),
	}, gomock.Any())
	m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetID),
		StackName:     aws.String(mockStack.Name),
	}).Return(&cloudformation.DescribeChangeSetOutput{
		Changes: []*cloudformation.Change{
			{
				ResourceChange: &cloudformation.ResourceChange{
					ResourceType: aws.String("ecs service"),
				},
				Type: aws.String(cloudformation.ChangeTypeResource),
			},
		},
		ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
		StatusReason:    aws.String("some reason"),
	}, nil)
	m.EXPECT().ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetID),
		StackName:     aws.String(mockStack.Name),
	})
}
