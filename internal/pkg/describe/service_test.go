// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	ecsapi "github.com/aws/aws-sdk-go/service/ecs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type ecsSvcDescriberMocks struct {
	mockCFN       *mocks.MockstackDescriber
	mockECSClient *mocks.MockecsClient
}

func TestServiceDescriber_EnvVars(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "svc"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedEnvVars []*awsecs.ContainerEnvVar
		wantedError   error
	}{
		"returns error if fails to get task definition": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				m.mockECSClient.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("describe task definition for service svc: some error"),
		},
		"get environment variables": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(&ecs.TaskDefinition{
						ContainerDefinitions: []*ecsapi.ContainerDefinition{
							{
								Name: aws.String("container"),
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
			mocks := ecsSvcDescriberMocks{
				mockECSClient: mockecsClient,
			}

			tc.setupMocks(mocks)

			d := &ECSServiceDescriber{
				ServiceDescriber: &ServiceDescriber{
					app:     testApp,
					service: testSvc,
					env:     testEnv,
				},
				ecsClient: mockecsClient,
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
		testSvc = "svc"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedSecrets []*awsecs.ContainerSecret
		wantedError   error
	}{
		"returns error if fails to get task definition": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("describe task definition for service svc: some error"),
		},
		"successfully gets secrets": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(&ecs.TaskDefinition{
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
			mocks := ecsSvcDescriberMocks{
				mockECSClient: mockecsClient,
			}

			tc.setupMocks(mocks)

			d := &ECSServiceDescriber{
				ServiceDescriber: &ServiceDescriber{
					app:     testApp,
					service: testSvc,
					env:     testEnv,
				},
				ecsClient: mockecsClient,
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
		testApp = "phonetool"
		testEnv = "test"
		testSvc = "jobs"
	)
	testdeployedSvcResources := []*stack.Resource{
		{
			Type:       "AWS::EC2::SecurityGroup",
			PhysicalID: "sg-0758ed6b233743530",
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedResources []*stack.Resource
		wantedError     error
	}{
		"returns error when fail to describe stack resources": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockCFN.EXPECT().Resources().Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockCFN.EXPECT().Resources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
						{
							Type:       "AWS::CloudFormation::WaitConditionHandle",
							PhysicalID: "https://cloudformation-waitcondition-us-west-2.s3-us-west-2.amazonaws.com/",
						},
						{
							Type:       "Custom::RulePriorityFunction",
							PhysicalID: "alb-rule-priority-HTTPRulePriorityAction",
						},
						{
							Type:       "AWS::CloudFormation::WaitCondition",
							PhysicalID: "arn:aws:cloudformation:us-west-2:1234567890",
						},
					}, nil),
				)
			},

			wantedResources: testdeployedSvcResources,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCFN := mocks.NewMockstackDescriber(ctrl)
			mocks := ecsSvcDescriberMocks{
				mockCFN: mockCFN,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,
				cfn:     mockCFN,
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
