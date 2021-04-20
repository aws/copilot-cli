// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"

	"github.com/aws/copilot-cli/internal/pkg/generator/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestServiceCommandGenerator_Generate(t *testing.T) {
	var (
		testApp = "app"
		testEnv = "env"
		testSvc = "svc"
	)
	testCases := map[string]struct {
		setUpMock func(m *mocks.MockecsInformationGetter)

		wantedGenerateCommandOpts *GenerateCommandOpts
		wantedError               error
	}{
		"returns generateCommandOpts with service's main container": {
			setUpMock: func(m *mocks.MockecsInformationGetter) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:       aws.String(testSvc),
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
						{
							Name: aws.String("random-container-that-we-do-not-care"),
						},
					},
				}, nil)
				m.EXPECT().NetworkConfiguration(testApp, testEnv, testSvc).Return(&ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				}, nil)
				m.EXPECT().ClusterARN(testApp, testEnv).Return("kamura-village", nil)
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

				cluster: "kamura-village",
			},
		},
		"unable to retrieve task definition": {
			setUpMock: func(m *mocks.MockecsInformationGetter) {
				m.EXPECT().TaskDefinition(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
				m.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedError: errors.New("retrieve task definition for service svc: some error"),
		},
		"unable to retrieve network configuration": {
			setUpMock: func(m *mocks.MockecsInformationGetter) {
				m.EXPECT().TaskDefinition(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().NetworkConfiguration(testApp, testEnv, testSvc).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("retrieve network configuration for service svc: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockecsInformationGetter(ctrl)
			tc.setUpMock(m)

			g := ServiceCommandGenerator{
				App:     testApp,
				Env:     testEnv,
				Service: testSvc,

				ECSInformationGetter: m,
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
