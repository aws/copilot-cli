// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const mockChangeSetName = "ecscli-31323334-3536-4738-b930-313233333435"

var (
	mockStack = NewStack("id", "template")

	errDoesNotExist = awserr.New("ValidationError", "does not exist", nil)
)

func TestCloudFormation_Create(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"fail if checking the stack description fails": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.name),
				}).Return(nil, errors.New("some unexpected error"))
				return m
			},
			wantedErr: fmt.Errorf("describe stack %s: %w", mockStack.name, errors.New("some unexpected error")),
		},
		"fail if a stack exists that's already in progress": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateInProgress),
						},
					},
				}, nil)
				return m
			},
			wantedErr: &errStackUpdateInProgress{
				name: mockStack.name,
			},
		},
		"fail if a successfully created stack already exists": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
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
				Name: mockStack.name,
				Stack: &cloudformation.Stack{
					StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
				},
			},
		},
		"creates the stack if it doesn't exist": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				addCreateDeployCalls(m)
				return m
			},
		},
		"creates the stack after cleaning the previously failed execution": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusRollbackComplete),
						},
					},
				}, nil)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.name),
				})
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
			err := c.Create(mockStack)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_CreateAndWait(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"waits until the stack is created": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errDoesNotExist)
				addCreateDeployCalls(m)
				m.EXPECT().WaitUntilStackCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.name),
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
			err := c.CreateAndWait(mockStack)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_Update(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"fail if the stack is already in progress": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusUpdateInProgress),
						},
					},
				}, nil)
				return m
			},
			wantedErr: &errStackUpdateInProgress{
				name: mockStack.name,
			},
		},
		"update a previously existing stack": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil)
				addUpdateDeployCalls(m)
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
			err := c.Update(mockStack)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_UpdateAndWait(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"waits until the stack is created": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
						},
					},
				}, nil)
				addUpdateDeployCalls(m)
				m.EXPECT().WaitUntilStackUpdateCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.name),
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
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"fails on unexpected error": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DeleteStack(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("delete stack %s: %w", mockStack.name, errors.New("some error")),
		},
		"exits successfully if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.name),
				}).Return(nil, errDoesNotExist)
				return m
			},
		},
		"exits successfully if stack can be deleted": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.name),
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
			err := c.Delete(mockStack.name)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_DeleteAndWait(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) api
		wantedErr  error
	}{
		"skip waiting if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DeleteStack(gomock.Any()).Return(nil, errDoesNotExist)
				m.EXPECT().WaitUntilStackDeleteCompleteWithContext(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				return m
			},
		},
		"wait for stack deletion if stack is being deleted": {
			createMock: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DeleteStack(&cloudformation.DeleteStackInput{
					StackName: aws.String(mockStack.name),
				}).Return(nil, nil)
				m.EXPECT().WaitUntilStackDeleteCompleteWithContext(gomock.Any(), &cloudformation.DescribeStacksInput{
					StackName: aws.String(mockStack.name),
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
			err := c.DeleteAndWait(mockStack.name)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func addCreateDeployCalls(m *mocks.Mockapi) {
	addDeployCalls(m, cloudformation.ChangeSetTypeCreate)
}

func addUpdateDeployCalls(m *mocks.Mockapi) {
	addDeployCalls(m, cloudformation.ChangeSetTypeUpdate)
}

func addDeployCalls(m *mocks.Mockapi, changeSetType string) {
	m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetName),
		StackName:     aws.String(mockStack.name),
		ChangeSetType: aws.String(changeSetType),
		TemplateBody:  aws.String(mockStack.template),
		Parameters:    nil,
		Tags:          nil,
		RoleARN:       nil,
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
			cloudformation.CapabilityCapabilityNamedIam,
		}),
	}).Return(nil, nil)
	m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetName),
		StackName:     aws.String(mockStack.name),
	}, gomock.Any())
	m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(mockChangeSetName),
		StackName:     aws.String(mockStack.name),
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
		ChangeSetName: aws.String(mockChangeSetName),
		StackName:     aws.String(mockStack.name),
	})
}
