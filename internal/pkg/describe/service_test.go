// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type svcDescriberMocks struct {
	mockStackDescriber *mocks.MockstackAndResourcesDescriber
	mockecsClient      *mocks.MockecsClient
}

func TestServiceDescriber_EnvVars(t *testing.T) {
	const (
		testApp            = "phonetool"
		testSvc            = "jobs"
		testEnv            = "test"
		testRegion         = "us-west-2"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedEnvVars []*ecs.ContainerEnvVar
		wantedError   error
	}{
		"returns error if fails to get environment variables": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockecsClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"get environment variables": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockecsClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
						ContainerDefinitions: []*ecsapi.ContainerDefinition{
							{
								Environment: []*ecsapi.KeyValuePair{
									{
										Name:  aws.String("COPILOT_SERVICE_NAME"),
										Value: aws.String("my-svc"),
									},
									{
										Name:  aws.String("COPILOT_ENVIRONMENT_NAME"),
										Value: aws.String("prod"),
									},
								},
								Name: aws.String("container"),
							},
						},
					}, nil),
				)
			},
			wantedEnvVars: []*ecs.ContainerEnvVar{
				{
					Name:      "COPILOT_SERVICE_NAME",
					Container: "container",
					Value:     "my-svc",
				},
				{
					Name:      "COPILOT_ENVIRONMENT_NAME",
					Container: "container",
					Value:     "prod",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockecsClient := mocks.NewMockecsClient(ctrl)
			mockStackDescriber := mocks.NewMockstackAndResourcesDescriber(ctrl)
			mocks := svcDescriberMocks{
				mockecsClient:      mockecsClient,
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

				ecsClient:      mockecsClient,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.EnvVars()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedEnvVars, actual)
			}
		})
	}
}

func TestServiceDescriber_Secrets(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "jobs"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedSecrets []*ecs.ContainerSecret
		wantedError   error
	}{
		"returns error if fails to get secrets": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockecsClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"successfully gets secrets": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockecsClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
						ContainerDefinitions: []*ecsapi.ContainerDefinition{
							{
								Name: aws.String("container"),
								Secrets: []*ecsapi.Secret{
									{
										Name:      aws.String("GITHUB_WEBHOOK_SECRET"),
										ValueFrom: aws.String("GH_WEBHOOK_SECRET"),
									},
									{
										Name:      aws.String("SOME_OTHER_SECRET"),
										ValueFrom: aws.String("SHHHHHHHH"),
									},
								},
							},
						},
					}, nil),
				)
			},
			wantedSecrets: []*ecs.ContainerSecret{
				{
					Name:      "GITHUB_WEBHOOK_SECRET",
					Container: "container",
					ValueFrom: "GH_WEBHOOK_SECRET",
				},
				{
					Name:      "SOME_OTHER_SECRET",
					Container: "container",
					ValueFrom: "SHHHHHHHH",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockecsClient := mocks.NewMockecsClient(ctrl)
			mockStackDescriber := mocks.NewMockstackAndResourcesDescriber(ctrl)
			mocks := svcDescriberMocks{
				mockecsClient:      mockecsClient,
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

				ecsClient:      mockecsClient,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.Secrets()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSecrets, actual)
			}
		})
	}
}

func TestServiceDescriber_ServiceStackResources(t *testing.T) {
	const (
		testApp            = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testSvc            = "jobs"
	)
	testCfnResources := []*cloudformation.StackResource{
		{
			ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
			PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedResources []*cloudformation.StackResource
		wantedError     error
	}{
		"returns error when fail to describe stack resources": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testApp, testEnv, testSvc)).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockStackDescriber.EXPECT().StackResources(stack.NameForService(testApp, testEnv, testSvc)).Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
							PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
						},
						{
							ResourceType:       aws.String("AWS::CloudFormation::WaitConditionHandle"),
							PhysicalResourceId: aws.String("https://cloudformation-waitcondition-us-west-2.s3-us-west-2.amazonaws.com/"),
						},
						{
							ResourceType:       aws.String("Custom::RulePriorityFunction"),
							PhysicalResourceId: aws.String("alb-rule-priority-HTTPRulePriorityAction"),
						},
						{
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
			mocks := svcDescriberMocks{
				mockStackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:            testApp,
				service:        testSvc,
				env:            testEnv,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			actual, err := d.ServiceStackResources()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedResources, actual)
			}
		})
	}
}
