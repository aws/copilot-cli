// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestService_TaskDefinition(t *testing.T) {
	mockError := errors.New("error")

	testCases := map[string]struct {
		taskDefinitionName string
		mockECSClient      func(m *mocks.MockecsClient)

		wantErr     error
		wantTaskDef *TaskDefinition
	}{
		"should return wrapped error given error": {
			taskDefinitionName: "task-def",
			mockECSClient: func(m *mocks.MockecsClient) {
				m.EXPECT().DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
					TaskDefinition: aws.String("task-def"),
				}).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("describe task definition %s: %w", "task-def", mockError),
		},
		"returns task definition given a task definition name": {
			taskDefinitionName: "task-def",
			mockECSClient: func(m *mocks.MockecsClient) {
				m.EXPECT().DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
					TaskDefinition: aws.String("task-def"),
				}).Return(&ecs.DescribeTaskDefinitionOutput{
					TaskDefinition: &ecs.TaskDefinition{
						ContainerDefinitions: []*ecs.ContainerDefinition{
							&ecs.ContainerDefinition{
								Environment: []*ecs.KeyValuePair{
									&ecs.KeyValuePair{
										Name:  aws.String("ECS_CLI_APP_NAME"),
										Value: aws.String("my-app"),
									},
									&ecs.KeyValuePair{
										Name:  aws.String("ECS_CLI_ENVIRONMENT_NAME"),
										Value: aws.String("prod"),
									},
								},
							},
						},
					},
				}, nil)
			},
			wantTaskDef: &TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					&ecs.ContainerDefinition{
						Environment: []*ecs.KeyValuePair{
							&ecs.KeyValuePair{
								Name:  aws.String("ECS_CLI_APP_NAME"),
								Value: aws.String("my-app"),
							},
							&ecs.KeyValuePair{
								Name:  aws.String("ECS_CLI_ENVIRONMENT_NAME"),
								Value: aws.String("prod"),
							},
						},
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

			mockECSClient := mocks.NewMockecsClient(ctrl)
			tc.mockECSClient(mockECSClient)

			service := Service{
				ecs: mockECSClient,
			}

			gotTaskDef, gotErr := service.TaskDefinition(tc.taskDefinitionName)

			if gotErr != nil {
				require.Equal(t, tc.wantErr, gotErr)
			} else {
				require.Equal(t, tc.wantTaskDef, gotTaskDef)
			}
		})

	}
}

func TestTaskDefinition_EnvVars(t *testing.T) {
	testCases := map[string]struct {
		inContainers []*ecs.ContainerDefinition

		wantEnvVars map[string]string
	}{
		"should return wrapped error given error": {
			inContainers: []*ecs.ContainerDefinition{
				&ecs.ContainerDefinition{
					Environment: []*ecs.KeyValuePair{
						&ecs.KeyValuePair{
							Name:  aws.String("ECS_CLI_APP_NAME"),
							Value: aws.String("my-app"),
						},
						&ecs.KeyValuePair{
							Name:  aws.String("ECS_CLI_ENVIRONMENT_NAME"),
							Value: aws.String("prod"),
						},
					},
				},
			},

			wantEnvVars: map[string]string{
				"ECS_CLI_APP_NAME":         "my-app",
				"ECS_CLI_ENVIRONMENT_NAME": "prod",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskDefinition := TaskDefinition{
				ContainerDefinitions: tc.inContainers,
			}

			gotEnvVars := taskDefinition.EnvironmentVariables()

			require.Equal(t, tc.wantEnvVars, gotEnvVars)
		})

	}
}
