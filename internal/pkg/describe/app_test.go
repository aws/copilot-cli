// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type appDescriberMocks struct {
	mockStackDescriber *mocks.MockstackAndResourcesDescriber
	mockEcsService     *mocks.MockecsService
}

func TestAppDescriber_EnvVars(t *testing.T) {
	const (
		testProject        = "phonetool"
		testApp            = "jobs"
		testEnv            = "test"
		testRegion         = "us-west-2"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
	)
	testCases := map[string]struct {
		project string
		app     string
		env     string

		setupMocks func(mocks appDescriberMocks)

		wantedEnvVars map[string]string
		wantedError   error
	}{
		"returns error if fails to get environment variables": {
			project: testProject,
			app:     testApp,
			env:     testEnv,
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockEcsService.EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"get environment variables": {
			project: testProject,
			app:     testApp,
			env:     testEnv,
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockEcsService.EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
						ContainerDefinitions: []*ecsapi.ContainerDefinition{
							&ecsapi.ContainerDefinition{
								Environment: []*ecsapi.KeyValuePair{
									&ecsapi.KeyValuePair{
										Name:  aws.String("COPILOT_SERVICE_NAME"),
										Value: aws.String("my-svc"),
									},
									&ecsapi.KeyValuePair{
										Name:  aws.String("COPILOT_ENVIRONMENT_NAME"),
										Value: aws.String("prod"),
									},
								},
							},
						},
					}, nil),
				)
			},
			wantedEnvVars: map[string]string{
				"COPILOT_SERVICE_NAME":     "my-svc",
				"COPILOT_ENVIRONMENT_NAME": "prod",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEcsService := mocks.NewMockecsService(ctrl)
			mockStackDescriber := mocks.NewMockstackAndResourcesDescriber(ctrl)
			mocks := appDescriberMocks{
				mockEcsService:     mockEcsService,
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &AppDescriber{
				project: tc.project,
				app:     tc.app,
				env:     tc.env,

				ecsClient:      mockEcsService,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.EnvVars()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedEnvVars, actual)
			}
		})
	}
}

func TestAppDescriber_AppStackResources(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
	)
	testCfnResources := []*cloudformation.StackResource{
		{
			ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
			PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks appDescriberMocks)

		wantedResources []*cloudformation.StackResource
		wantedError     error
	}{
		"returns error when fail to describe stack resources": {
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testProject, testEnv, testApp)).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testProject, testEnv, testApp)).Return([]*cloudformation.StackResource{
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

			mockStackDescriber := mocks.NewMockstackAndResourcesDescriber(ctrl)
			mocks := appDescriberMocks{
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &AppDescriber{
				project:        testProject,
				app:            testApp,
				env:            testEnv,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.AppStackResources()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.wantedResources, actual)
			}
		})
	}
}

func TestAppDescriber_GetServiceArn(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
	)
	mockServiceArn := ecs.ServiceArn("mockServiceArn")
	testCases := map[string]struct {
		setupMocks func(mocks appDescriberMocks)

		wantedServiceArn *ecs.ServiceArn
		wantedError      error
	}{
		"success": {
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testProject, testEnv, testApp)).
						Return([]*cloudformation.StackResource{
							{
								LogicalResourceId:  aws.String("Service"),
								PhysicalResourceId: aws.String("mockServiceArn"),
							},
						}, nil),
				)
			},

			wantedServiceArn: &mockServiceArn,
		},
		"error if cannot find service arn": {
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testProject, testEnv, testApp)).
						Return([]*cloudformation.StackResource{}, nil),
				)
			},

			wantedError: fmt.Errorf("cannot find service arn in app stack resource"),
		},
		"error if fail to describe stack resources": {
			setupMocks: func(m appDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testProject, testEnv, testApp)).
						Return(nil, errors.New("some error")),
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

			mockStackDescriber := mocks.NewMockstackAndResourcesDescriber(ctrl)
			mocks := appDescriberMocks{
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &AppDescriber{
				project:        testProject,
				app:            testApp,
				env:            testEnv,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.GetServiceArn()

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
