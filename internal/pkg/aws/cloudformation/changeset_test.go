// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestChangeSet_createAndExecute(t *testing.T) {
	const (
		mockStackName     = "phonetool"
		mockChangeSetName = "mockChangeSet"
	)
	testCases := map[string]struct {
		inConfig      stackConfig
		changesetMock func(ctrl *gomock.Controller) changeSetAPI
		wantedErr     error
	}{
		"error if fail to create the changeset because of random issue": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
				}).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(&cloudformation.DescribeChangeSetOutput{
						Changes:      []*cloudformation.Change{},
						StatusReason: aws.String("some other reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("create change set mockChangeSet for stack phonetool: some error: some other reason"),
		},
		"error if fail to wait until the changeset creation complete": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(&cloudformation.DescribeChangeSetOutput{
						StatusReason: aws.String("some reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("wait for creation of change set mockChangeSet for stack phonetool: some error: some reason"),
		},
		"error if fail to describe change set after creation failed": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
				}).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
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
			wantedErr: fmt.Errorf("check if changeset is empty: create change set mockChangeSet for stack phonetool: some error: describe change set mockChangeSet for stack phonetool: some error"),
		},
		"delete change set and throw ErrChangeSetEmpty if failed to create the change set because it is empty": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
				}).Return(nil, errors.New("some error"))
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
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
			wantedErr: fmt.Errorf("change set with name mockChangeSet for stack phonetool has no changes"),
		},
		"error if creation succeed but failed to describe change set": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
				}).Return(&cloudformation.CreateChangeSetOutput{
					Id: aws.String(mockChangeSetName),
				}, nil)
				m.EXPECT().WaitUntilChangeSetCreateCompleteWithContext(gomock.Any(), &cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
				}, gomock.Any()).Return(nil)
				m.EXPECT().DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
					ChangeSetName: aws.String(mockChangeSetName),
					StackName:     aws.String(mockStackName)}).
					Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("describe change set mockChangeSet for stack phonetool: some error"),
		},
		"ignore execute request if the change set does not contain any modifications": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
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
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
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
						StatusReason:    aws.String("some other reason"),
					}, nil)
				return m
			},
			wantedErr: fmt.Errorf("execute change set mockChangeSet for stack phonetool because status is UNAVAILABLE with reason some other reason"),
		},
		"error if fail to execute change set": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
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
				}).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("execute change set mockChangeSet for stack phonetool: some error"),
		},
		"success": {
			inConfig: stackConfig{
				TemplateBody: "some body",
			},
			changesetMock: func(ctrl *gomock.Controller) changeSetAPI {
				m := mocks.NewMockchangeSetAPI(ctrl)
				m.EXPECT().CreateChangeSet(&cloudformation.CreateChangeSetInput{
					ChangeSetName:       aws.String(mockChangeSetName),
					StackName:           aws.String(mockStackName),
					ChangeSetType:       aws.String("CREATE"),
					IncludeNestedStacks: aws.Bool(true),
					Capabilities: aws.StringSlice([]string{
						cloudformation.CapabilityCapabilityIam,
						cloudformation.CapabilityCapabilityNamedIam,
						cloudformation.CapabilityCapabilityAutoExpand,
					}),
					TemplateBody: aws.String("some body"),
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
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := &changeSet{
				name:      mockChangeSetName,
				stackName: mockStackName,
				csType:    createChangeSetType,

				client: tc.changesetMock(ctrl),
			}
			// WHEN
			err := c.createAndExecute(&tc.inConfig)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
