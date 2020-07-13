// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const testName = "stackset"

var testError = errors.New("some error")

func TestStackSet_Create(t *testing.T) {
	const (
		testTemplate           = "body"
		testDescription        = "amazing stack set"
		testExecutionRole      = "execARN"
		testAdministrationRole = "adminARN"
	)
	testTags := map[string]string{
		"owner": "boss",
	}
	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedError error
	}{
		"succeeds if new stack set": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().CreateStackSet(&cloudformation.CreateStackSetInput{
					AdministrationRoleARN: aws.String(testAdministrationRole),
					Description:           aws.String(testDescription),
					ExecutionRoleName:     aws.String(testExecutionRole),
					StackSetName:          aws.String(testName),
					Tags: []*cloudformation.Tag{
						{
							Key:   aws.String("owner"),
							Value: aws.String("boss"),
						},
					},
					TemplateBody: aws.String(testTemplate),
				})
				return m
			},
		},
		"succeeds if stack set already exists": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().CreateStackSet(gomock.Any()).Return(nil, awserr.New(cloudformation.ErrCodeNameAlreadyExistsException, "", nil))
				return m
			},
		},
		"wraps error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().CreateStackSet(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("create stack set %s: %w", testName, testError),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			err := client.Create(testName, testTemplate,
				WithDescription(testDescription),
				WithAdministrationRoleARN(testAdministrationRole),
				WithExecutionRoleName(testExecutionRole),
				WithTags(testTags))

			// THEN
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_Describe(t *testing.T) {
	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedDescr Description
		wantedError error
	}{
		"succeeds": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStackSet(&cloudformation.DescribeStackSetInput{
					StackSetName: aws.String(testName),
				}).Return(&cloudformation.DescribeStackSetOutput{
					StackSet: &cloudformation.StackSet{
						StackSetId:   aws.String(testName),
						StackSetName: aws.String(testName),
						TemplateBody: aws.String("body"),
					},
				}, nil)
				return m
			},
			wantedDescr: Description{
				ID:       testName,
				Name:     testName,
				Template: "body",
			},
		},
		"wraps error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().DescribeStackSet(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("describe stack set %s: %w", testName, testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			descr, err := client.Describe(testName)

			// THEN
			require.Equal(t, tc.wantedDescr, descr)
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_Update(t *testing.T) {
	const (
		testTemplate           = "body"
		testDescription        = "amazing stack set"
		testExecutionRole      = "execARN"
		testAdministrationRole = "adminARN"
		testOperationID        = "2"
	)
	var (
		testTags = map[string]string{
			"owner": "boss",
		}
		errOpExists = awserr.New(cloudformation.ErrCodeOperationIdAlreadyExistsException, "", nil)
		errOpInProg = awserr.New(cloudformation.ErrCodeOperationInProgressException, "", nil)
		errOpStale  = awserr.New(cloudformation.ErrCodeStaleRequestException, "", nil)
	)

	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedError error
	}{
		"updates stack with operation is valid": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(&cloudformation.UpdateStackSetInput{
					OperationId:           aws.String(testOperationID),
					AdministrationRoleARN: aws.String(testAdministrationRole),
					Description:           aws.String(testDescription),
					ExecutionRoleName:     aws.String(testExecutionRole),
					StackSetName:          aws.String(testName),
					Tags: []*cloudformation.Tag{
						{
							Key:   aws.String("owner"),
							Value: aws.String("boss"),
						},
					},
					TemplateBody: aws.String(testTemplate),
				}).Return(&cloudformation.UpdateStackSetOutput{
					OperationId: aws.String(testOperationID),
				}, nil)
				return m
			},
		},
		"returns ErrStackSetOutOfDate if operation exists already": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(nil, errOpExists)
				return m
			},
			wantedError: &ErrStackSetOutOfDate{
				stackSetName: testName,
				parentErr:    errOpExists,
			},
		},
		"returns ErrStackSetOutOfDate if operation in progress": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(nil, errOpInProg)
				return m
			},
			wantedError: &ErrStackSetOutOfDate{
				stackSetName: testName,
				parentErr:    errOpInProg,
			},
		},
		"returns ErrStackSetOutOfDate if operation is stale": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(nil, errOpStale)
				return m
			},
			wantedError: &ErrStackSetOutOfDate{
				stackSetName: testName,
				parentErr:    errOpStale,
			},
		},
		"wrap error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("update stack set %s: %w", testName, testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			err := client.Update(testName, testTemplate,
				WithOperationID(testOperationID),
				WithDescription(testDescription),
				WithAdministrationRoleARN(testAdministrationRole),
				WithExecutionRoleName(testExecutionRole),
				WithTags(testTags))

			// THEN
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_UpdateAndWait(t *testing.T) {
	const testTemplate = "body"
	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedError error
	}{
		"waits until operation succeeds": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(&cloudformation.UpdateStackSetOutput{
					OperationId: aws.String("1"),
				}, nil)
				m.EXPECT().DescribeStackSetOperation(&cloudformation.DescribeStackSetOperationInput{
					StackSetName: aws.String(testName),
					OperationId:  aws.String("1"),
				}).Return(&cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String(opStatusSucceeded),
					},
				}, nil)
				return m
			},
		},
		"returns a wrapped error if operation stopped": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(&cloudformation.UpdateStackSetOutput{
					OperationId: aws.String("1"),
				}, nil)
				m.EXPECT().DescribeStackSetOperation(gomock.Any()).Return(&cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String(opStatusStopped),
					},
				}, nil)
				return m
			},
			wantedError: fmt.Errorf("operation %s for stack set %s was manually stopped", "1", testName),
		},
		"returns a wrapped error if operation failed": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().UpdateStackSet(gomock.Any()).Return(&cloudformation.UpdateStackSetOutput{
					OperationId: aws.String("1"),
				}, nil)
				m.EXPECT().DescribeStackSetOperation(gomock.Any()).Return(&cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String(opStatusFailed),
					},
				}, nil)
				return m
			},
			wantedError: fmt.Errorf("operation %s for stack set %s failed", "1", testName),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			err := client.UpdateAndWait(testName, testTemplate)

			// THEN
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_Delete(t *testing.T) {
	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedError error
	}{
		"do nothing if stack set does not exist": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(nil, awserr.New(cloudformation.ErrCodeStackSetNotFoundException, "", nil))
				return m
			},
		},
		"successfully deletes stack instances then stack set": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(&cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{
						{
							Account: aws.String("1234"),
							Region:  aws.String("us-east-1"),
						},
					},
				}, nil)
				m.EXPECT().DeleteStackInstances(&cloudformation.DeleteStackInstancesInput{
					StackSetName: aws.String(testName),
					Accounts:     aws.StringSlice([]string{"1234"}),
					Regions:      aws.StringSlice([]string{"us-east-1"}),
					RetainStacks: aws.Bool(false),
				}).Return(&cloudformation.DeleteStackInstancesOutput{
					OperationId: aws.String("1"),
				}, nil)
				m.EXPECT().DescribeStackSetOperation(gomock.Any()).Return(&cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String(opStatusSucceeded),
					},
				}, nil)
				m.EXPECT().DeleteStackSet(&cloudformation.DeleteStackSetInput{
					StackSetName: aws.String(testName),
				}).Return(nil, nil)
				return m
			},
		},
		"successfully exits if stack set does not exist": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(&cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{},
				}, nil)
				m.EXPECT().DeleteStackSet(gomock.Any()).Return(nil, awserr.New(cloudformation.ErrCodeStackSetNotFoundException, "", nil))
				return m
			},
		},
		"successfully exits if deletes stack set": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(&cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{},
				}, nil)
				m.EXPECT().DeleteStackSet(gomock.Any()).Return(nil, nil)
				return m
			},
		},
		"wraps error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(&cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{},
				}, nil)
				m.EXPECT().DeleteStackSet(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("delete stack set %s: %w", testName, testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			err := client.Delete(testName)

			// THEN
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_CreateInstances(t *testing.T) {
	var (
		testAccounts = []string{"1234"}
		testRegions  = []string{"us-west-1"}
	)
	testCases := map[string]struct {
		mockClient  func(ctrl *gomock.Controller) api
		wantedError error
	}{
		"successfully creates stack instances": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().CreateStackInstances(&cloudformation.CreateStackInstancesInput{
					StackSetName: aws.String(testName),
					Accounts:     aws.StringSlice(testAccounts),
					Regions:      aws.StringSlice(testRegions),
				}).Return(&cloudformation.CreateStackInstancesOutput{
					OperationId: aws.String("1"),
				}, nil)
				return m
			},
		},
		"wraps error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().CreateStackInstances(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("create stack instances for stack set %s in regions %v for accounts %v: %w",
				testName,
				testRegions,
				testAccounts,
				testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			err := client.CreateInstances(testName, testAccounts, testRegions)

			// THEN
			require.Equal(t, tc.wantedError, err)
		})
	}
}

func TestStackSet_InstanceSummaries(t *testing.T) {
	const (
		testAccountID = "1234"
		testRegion    = "us-west-1"
	)
	testCases := map[string]struct {
		mockClient      func(ctrl *gomock.Controller) api
		wantedSummaries []InstanceSummary
		wantedError     error
	}{
		"returns summaries": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(&cloudformation.ListStackInstancesInput{
					StackSetName:         aws.String(testName),
					StackInstanceAccount: aws.String(testAccountID),
					StackInstanceRegion:  aws.String(testRegion),
				}).Return(&cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{
						{
							StackId: aws.String(testName),
							Account: aws.String(testAccountID),
							Region:  aws.String(testRegion),
						},
					},
				}, nil)
				return m
			},
			wantedSummaries: []InstanceSummary{
				{
					StackID: testName,
					Account: testAccountID,
					Region:  testRegion,
				},
			},
		},
		"wraps error on unexpected failure": {
			mockClient: func(ctrl *gomock.Controller) api {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().ListStackInstances(gomock.Any()).Return(nil, testError)
				return m
			},
			wantedError: fmt.Errorf("list stack instances for stack set %s: %w", testName, testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := StackSet{
				client: tc.mockClient(ctrl),
			}

			// WHEN
			summaries, err := client.InstanceSummaries(
				testName,
				FilterSummariesByAccountID(testAccountID),
				FilterSummariesByRegion(testRegion))

			// THEN
			require.ElementsMatch(t, tc.wantedSummaries, summaries)
			require.Equal(t, tc.wantedError, err)
		})
	}
}
