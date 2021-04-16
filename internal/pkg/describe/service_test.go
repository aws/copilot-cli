// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type svcDescriberMocks struct {
	mockCFN              *mocks.Mockcfn
	mockECSClient        *mocks.MockecsClient
	mockClusterDescriber *mocks.MockclusterDescriber
}

func TestServiceDescriber_EnvVars(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "jobs"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedEnvVars []*awsecs.ContainerEnvVar
		wantedError   error
	}{
		"returns error if fails to get environment variables": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"get environment variables": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
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
			mocks := svcDescriberMocks{
				mockECSClient: mockecsClient,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

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
		testSvc = "jobs"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedSecrets []*awsecs.ContainerSecret
		wantedError   error
	}{
		"returns error if fails to get secrets": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"successfully gets secrets": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockECSClient.EXPECT().TaskDefinition("phonetool-test-jobs").Return(&ecs.TaskDefinition{
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
			mocks := svcDescriberMocks{
				mockECSClient: mockecsClient,
			}

			tc.setupMocks(mocks)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

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
					m.mockCFN.EXPECT().StackResources(stack.NameForService(testApp, testEnv, testSvc)).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m svcDescriberMocks) {
				gomock.InOrder(
					m.mockCFN.EXPECT().StackResources(stack.NameForService(testApp, testEnv, testSvc)).Return([]*cloudformation.StackResource{
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

			mockCFN := mocks.NewMockcfn(ctrl)
			mocks := svcDescriberMocks{
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

func TestServiceDescriber_NetworkConfiguration(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "svc"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedNetworkConfig *awsecs.NetworkConfiguration
		wantedError         error
	}{
		"unable to get cluster ARN": {
			setupMocks: func(mocks svcDescriberMocks) {
				mocks.mockClusterDescriber.EXPECT().ClusterARN(testApp, testEnv).Return("", errors.New("some error"))
			},
			wantedError: errors.New("get cluster ARN for service svc: some error"),
		},
		"successfully retrieve network configuration": {
			setupMocks: func(mocks svcDescriberMocks) {
				mocks.mockClusterDescriber.EXPECT().ClusterARN(testApp, testEnv).Return("cluster-1", nil)
				mocks.mockECSClient.EXPECT().NetworkConfiguration("cluster-1", testSvc).Return(&awsecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					SecurityGroups: []string{"sg-1", "sg-2"},
					Subnets:        []string{"sn-1", "sn-2"},
				}, nil)
			},
			wantedNetworkConfig: &awsecs.NetworkConfiguration{
				AssignPublicIp: "1.2.3.4",
				SecurityGroups: []string{"sg-1", "sg-2"},
				Subnets:        []string{"sn-1", "sn-2"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcDescriberMocks{
				mockECSClient:        mocks.NewMockecsClient(ctrl),
				mockClusterDescriber: mocks.NewMockclusterDescriber(ctrl),
			}

			tc.setupMocks(m)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

				ecsClient:        m.mockECSClient,
				clusterDescriber: m.mockClusterDescriber,
			}

			// WHEN
			actual, err := d.NetworkConfiguration()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedNetworkConfig, actual)
			}
		})
	}
}

func TestServiceDescriber_TaskDefinition(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "svc"
		testEnv = "test"
	)
	testCases := map[string]struct {
		setupMocks func(mocks svcDescriberMocks)

		wantedTaskDefinition *TaskDefinition
		wantedError          error
	}{
		"unable to retrieve task definition": {
			setupMocks: func(mocks svcDescriberMocks) {
				mocks.mockECSClient.EXPECT().TaskDefinition("phonetool-test-svc").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get task definition phonetool-test-svc of service svc: some error"),
		},
		"successfully return task definition information": {
			setupMocks: func(mocks svcDescriberMocks) {
				mocks.mockECSClient.EXPECT().TaskDefinition("phonetool-test-svc").Return(&awsecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*ecsapi.ContainerDefinition{
						{
							Name:  aws.String("the-container"),
							Image: aws.String("beautiful-image"),
							Environment: []*ecsapi.KeyValuePair{
								{
									Name:  aws.String("weather"),
									Value: aws.String("snowy"),
								},
								{
									Name:  aws.String("temperature"),
									Value: aws.String("low"),
								},
							},
							Secrets: []*ecsapi.Secret{
								{
									Name:      aws.String("secret-1"),
									ValueFrom: aws.String("first walk to Hokkaido"),
								},
								{
									Name:      aws.String("secret-2"),
									ValueFrom: aws.String("then get on the HAYABUSA"),
								},
							},
							EntryPoint: aws.StringSlice([]string{"do", "not", "enter"}),
							Command:    aws.StringSlice([]string{"--force", "--verbose"}),
						},
					},
				}, nil)
			},
			wantedTaskDefinition: &TaskDefinition{
				Images: []*awsecs.ContainerImage{
					{
						Container: "the-container",
						Image:     "beautiful-image",
					},
				},
				ExecutionRole: "execution-role",
				TaskRole:      "task-role",
				EnvVars: []*awsecs.ContainerEnvVar{
					{
						Container: "the-container",
						Name:      "weather",
						Value:     "snowy",
					},
					{
						Container: "the-container",
						Name:      "temperature",
						Value:     "low",
					},
				},
				Secrets: []*awsecs.ContainerSecret{
					{
						Container: "the-container",
						Name:      "secret-1",
						ValueFrom: "first walk to Hokkaido",
					},
					{
						Container: "the-container",
						Name:      "secret-2",
						ValueFrom: "then get on the HAYABUSA",
					},
				},
				EntryPoints: []*awsecs.ContainerEntrypoint{
					{
						Container:  "the-container",
						EntryPoint: []string{"do", "not", "enter"},
					},
				},
				Commands: []*awsecs.ContainerCommand{
					{
						Container: "the-container",
						Command:   []string{"--force", "--verbose"},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcDescriberMocks{
				mockECSClient: mocks.NewMockecsClient(ctrl),
			}

			tc.setupMocks(m)

			d := &ServiceDescriber{
				app:     testApp,
				service: testSvc,
				env:     testEnv,

				ecsClient: m.mockECSClient,
			}

			// WHEN
			actual, err := d.TaskDefinition()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTaskDefinition, actual)
			}
		})
	}
}
