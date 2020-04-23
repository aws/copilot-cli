// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/stack/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	ECSAPI "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type describeMocks struct {
	mockEcsService     map[string]*mocks.MockecsService
	mockStackDescriber map[string]*mocks.MockstackDescriber
	mockStore          *mocks.MockstoreSvc
}

func TestDescriber_EnvVars(t *testing.T) {
	const (
		testProject        = "phonetool"
		testApp            = "jobs"
		testEnv            = "test"
		testRegion         = "us-west-2"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
	)
	testEnvironment := &archer.Environment{
		Name:           testEnv,
		ManagerRoleARN: testManagerRoleARN,
		Region:         testRegion,
	}
	testCases := map[string]struct {
		project     string
		app         string
		environment *archer.Environment

		setupMocks func(mocks describeMocks)

		wantedEnvVars []*EnvVars
		wantedError   error
	}{
		"returns error if fails to get environment variables": {
			project:     testProject,
			app:         testApp,
			environment: testEnvironment,
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockEcsService[testManagerRoleARN].EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"get environment variables": {
			project:     testProject,
			app:         testApp,
			environment: testEnvironment,
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockEcsService[testManagerRoleARN].EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
						ContainerDefinitions: []*ECSAPI.ContainerDefinition{
							&ECSAPI.ContainerDefinition{
								Environment: []*ECSAPI.KeyValuePair{
									&ECSAPI.KeyValuePair{
										Name:  aws.String("ECS_CLI_APP_NAME"),
										Value: aws.String("my-app"),
									},
									&ECSAPI.KeyValuePair{
										Name:  aws.String("ECS_CLI_ENVIRONMENT_NAME"),
										Value: aws.String("prod"),
									},
								},
							},
						},
					}, nil),
				)
			},

			wantedEnvVars: []*EnvVars{
				{
					Environment: "test",
					Name:        "ECS_CLI_ENVIRONMENT_NAME",
					Value:       "prod",
				},
				{
					Environment: "test",
					Name:        "ECS_CLI_APP_NAME",
					Value:       "my-app",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEcsService := mocks.NewMockecsService(ctrl)
			mocks := describeMocks{
				mockEcsService: map[string]*mocks.MockecsService{
					testManagerRoleARN: mockEcsService,
				},
			}

			tc.setupMocks(mocks)

			d := &Describer{
				project: tc.project,
				ecsClient: map[string]ecsService{
					testManagerRoleARN: mockEcsService,
				},
			}

			// WHEN
			actual, err := d.EnvVars(tc.environment, tc.app)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.wantedEnvVars, actual)
			}
		})
	}
}

func TestWebAppDescriber_StackResources(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
	)
	testCfnResources := []*CfnResource{
		&CfnResource{
			Type:       "AWS::EC2::SecurityGroup",
			PhysicalID: "sg-0758ed6b233743530",
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks describeMocks)

		wantedResources []*CfnResource
		wantedError     error
	}{
		"returns error when fail to describe stack resources": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("describe resources for stack phonetool-test-jobs: some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(&cloudformation.DescribeStackResourcesOutput{
						StackResources: []*cloudformation.StackResource{
							&cloudformation.StackResource{
								ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
								PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
							},
							&cloudformation.StackResource{
								ResourceType:       aws.String("AWS::CloudFormation::WaitConditionHandle"),
								PhysicalResourceId: aws.String("https://cloudformation-waitcondition-us-west-2.s3-us-west-2.amazonaws.com/"),
							},
							&cloudformation.StackResource{
								ResourceType:       aws.String("Custom::RulePriorityFunction"),
								PhysicalResourceId: aws.String("alb-rule-priority-HTTPRulePriorityAction"),
							},
							&cloudformation.StackResource{
								ResourceType:       aws.String("AWS::CloudFormation::WaitCondition"),
								PhysicalResourceId: aws.String(" arn:aws:cloudformation:us-west-2:1234567890"),
							},
						},
					}, nil),
				)
			},

			wantedResources: testCfnResources,
		},
		"get stack resources": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(&cloudformation.DescribeStackResourcesOutput{
						StackResources: []*cloudformation.StackResource{
							&cloudformation.StackResource{
								ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
								PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
							},
						},
					}, nil),
				)
			},

			wantedResources: testCfnResources,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreSvc := mocks.NewMockstoreSvc(ctrl)
			mockStackDescriber := mocks.NewMockstackDescriber(ctrl)
			mocks := describeMocks{
				mockStackDescriber: map[string]*mocks.MockstackDescriber{
					testManagerRoleARN: mockStackDescriber,
				},
				mockStore: mockStoreSvc,
			}

			tc.setupMocks(mocks)

			d := &Describer{
				project: testProject,
				store:   mockStoreSvc,
				stackDescribers: map[string]stackDescriber{
					testManagerRoleARN: mockStackDescriber,
				},
			}

			// WHEN
			actual, err := d.StackResources(testEnv, testApp)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedResources, actual)
			}
		})
	}
}

func TestWebAppDescriber_GetServiceArn(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
	)
	mockServiceArn := ecs.ServiceArn("mockServiceArn")
	testCases := map[string]struct {
		setupMocks func(mocks describeMocks)

		wantedServiceArn *ecs.ServiceArn
		wantedError      error
	}{
		"success": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(&cloudformation.DescribeStackResourcesOutput{
						StackResources: []*cloudformation.StackResource{
							{
								LogicalResourceId:  aws.String("Service"),
								PhysicalResourceId: aws.String("mockServiceArn"),
							},
						},
					}, nil),
				)
			},

			wantedServiceArn: &mockServiceArn,
		},
		"errors if cannot find service arn": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(&cloudformation.DescribeStackResourcesOutput{
						StackResources: []*cloudformation.StackResource{},
					}, nil),
				)
			},

			wantedError: fmt.Errorf("cannot find service arn in app stack resource"),
		},
		"errors if failed to describe stack resources": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
						Project:        testProject,
						Name:           testEnv,
						ManagerRoleARN: testManagerRoleARN,
					}, nil),
					m.mockStackDescriber[testManagerRoleARN].EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("describe resources for stack phonetool-test-jobs: some error"),
		},
		"errors if failed to get environment": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.mockStore.EXPECT().GetEnvironment(testProject, testEnv).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreSvc := mocks.NewMockstoreSvc(ctrl)
			mockStackDescriber := mocks.NewMockstackDescriber(ctrl)
			mocks := describeMocks{
				mockStackDescriber: map[string]*mocks.MockstackDescriber{
					testManagerRoleARN: mockStackDescriber,
				},
				mockStore: mockStoreSvc,
			}

			tc.setupMocks(mocks)

			d := &Describer{
				project: testProject,
				store:   mockStoreSvc,
				stackDescribers: map[string]stackDescriber{
					testManagerRoleARN: mockStackDescriber,
				},
			}

			// WHEN
			actual, err := d.GetServiceArn(testEnv, testApp)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedServiceArn, actual)
			}
		})
	}
}
