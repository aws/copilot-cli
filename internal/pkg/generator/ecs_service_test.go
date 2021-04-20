// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"

	awsecs "github.com/aws/aws-sdk-go/service/ecs"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/generator/mocks"
)

func TestECSServiceCommandGenerator_Generate(t *testing.T) {
	var (
		testCluster = "crowded-cluster"
		testService = "good-service"
	)
	testCases := map[string]struct {
		setUpMock func(m *mocks.MockecsServiceGetter)

		wantedGenerateCommandOpts *GenerateCommandOpts
		wantedError               error
	}{
		"success": {
			setUpMock: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:       aws.String("the-one-and-only-one-container"),
							Image:      aws.String("beautiful-image"),
							EntryPoint: aws.StringSlice([]string{"enter", "here"}),
							Command:    aws.StringSlice([]string{"do", "not", "enter", "here"}),
							Environment: []*awsecs.KeyValuePair{
								{
									Name:  aws.String("enter"),
									Value: aws.String("no"),
								},
								{
									Name:  aws.String("kidding"),
									Value: aws.String("yes"),
								},
							},
							Secrets: []*awsecs.Secret{
								{
									Name:      aws.String("truth"),
									ValueFrom: aws.String("go-ask-the-wise"),
								},
							},
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(testCluster, testService).Return(&ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				}, nil)
			},
			wantedGenerateCommandOpts: &GenerateCommandOpts{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				},

				executionRole: "execution-role",
				taskRole:      "task-role",

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "here"},
					command:    []string{"do", "not", "enter", "here"},
					envVars: map[string]string{
						"enter":   "no",
						"kidding": "yes",
					},
					secrets: map[string]string{
						"truth": "go-ask-the-wise",
					},
				},

				cluster: testCluster,
			},
		},
		"unable to retrieve service": {
			setUpMock: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service(testCluster, testService).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve service good-service in cluster crowded-cluster: some error"),
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition task-def: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().TaskDefinition(gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testCluster, testService).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("retrieve network configuration for service good-service in cluster crowded-cluster: some error"),
		},
		"error if found more than one container": {
			setUpMock: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service(testCluster, testService).Return(&ecs.Service{
					TaskDefinition: aws.String("task-def"),
				}, nil)
				m.EXPECT().TaskDefinition("task-def").Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name: aws.String("the-first-container"),
						},
						{
							Name: aws.String("sad-container"),
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("found more that one container in task definition: task-def"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockecsServiceGetter(ctrl)
			tc.setUpMock(m)

			g := ECSServiceCommandGenerator{
				Cluster:          testCluster,
				Service:          testService,
				ECSServiceGetter: m,
			}

			got, err := g.Generate()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedGenerateCommandOpts, got)
			}
		})
	}
}
