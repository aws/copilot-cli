// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type clientMocks struct {
	resourceGetter *mocks.MockresourceGetter
	ecsClient      *mocks.MockecsClient
	StepFuncClient *mocks.MockstepFunctionsClient
}

func TestClient_ClusterARN(t *testing.T) {
	const (
		mockApp = "mockApp"
		mockEnv = "mockEnv"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey: mockApp,
		deploy.EnvTagKey: mockEnv,
	}
	testError := errors.New("some error")

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError   error
		wantedCluster string
	}{
		"errors if fail to get resources by tags": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("get cluster resources for environment mockEnv: some error"),
		},
		"errors if no cluster found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{}, nil),
				)
			},
			wantedError: fmt.Errorf("no cluster found in environment mockEnv"),
		},
		"errors if more than one cluster found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockARN1"}, {ARN: "mockARN2"},
						}, nil),
				)
			},
			wantedError: fmt.Errorf("more than one cluster is found in environment mockEnv"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockARN"},
						}, nil),
				)
			},
			wantedCluster: "mockARN",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter: mockRgGetter,
			}

			// WHEN
			get, err := client.ClusterARN(mockApp, mockEnv)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, test.wantedCluster)
			}
		})
	}
}

func TestClient_serviceARN(t *testing.T) {
	const (
		mockApp = "mockApp"
		mockEnv = "mockEnv"
		mockSvc = "mockSvc"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}
	testError := errors.New("some error")

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError   error
		wantedService string
	}{
		"errors if fail to get resources by tags": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("get ECS service with tags (mockApp, mockEnv, mockSvc): some error"),
		},
		"errors if no service found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{}, nil),
				)
			},
			wantedError: fmt.Errorf("no ECS service found for mockSvc in environment mockEnv"),
		},
		"errors if more than one service found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockARN1"}, {ARN: "mockARN2"},
						}, nil),
				)
			},
			wantedError: fmt.Errorf("more than one ECS service with the name mockSvc found in environment mockEnv"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockARN"},
						}, nil),
				)
			},
			wantedService: "mockARN",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter: mockRgGetter,
			}

			// WHEN
			get, err := client.serviceARN(mockApp, mockEnv, mockSvc)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, string(*get), test.wantedService)
			}
		})
	}
}

func TestClient_DescribeService(t *testing.T) {
	const (
		mockApp     = "mockApp"
		mockEnv     = "mockEnv"
		mockSvc     = "mockSvc"
		badSvcARN   = "badMockArn"
		mockSvcARN  = "arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService"
		mockCluster = "mockCluster"
		mockService = "mockService"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError error
		wanted      *ServiceDesc
	}{
		"return error if failed to get cluster name": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: badSvcARN},
						}, nil),
				)
			},
			wantedError: fmt.Errorf("get cluster name: arn: invalid prefix"),
		},
		"return error if failed to get service tasks": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().ServiceRunningTasks(mockCluster, mockService).Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("get tasks for service mockService: some error"),
		},
		"return error if failed to get stopped service tasks": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().ServiceRunningTasks(mockCluster, mockService).Return([]*ecs.Task{
						{TaskArn: aws.String("mockTaskARN")},
					}, nil),
					m.ecsClient.EXPECT().StoppedServiceTasks(mockCluster, mockService).Return(nil, errors.New("some error")),
				)
			},
			wantedError: errors.New("get stopped tasks for service mockService: some error"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().ServiceRunningTasks(mockCluster, mockService).Return([]*ecs.Task{
						{TaskArn: aws.String("mockTaskARN")},
					}, nil),
					m.ecsClient.EXPECT().StoppedServiceTasks(mockCluster, mockService).Return([]*ecs.Task{
						{TaskArn: aws.String("mockStoppedTaskARN")},
					}, nil),
				)
			},
			wanted: &ServiceDesc{
				ClusterName: mockCluster,
				Name:        mockService,
				Tasks: []*ecs.Task{
					{TaskArn: aws.String("mockTaskARN")},
				},
				StoppedTasks: []*ecs.Task{
					{TaskArn: aws.String("mockStoppedTaskARN")},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mockECSClient := mocks.NewMockecsClient(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
				ecsClient:      mockECSClient,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter:  mockRgGetter,
				ecsClient: mockECSClient,
			}

			// WHEN
			get, err := client.DescribeService(mockApp, mockEnv, mockSvc)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, test.wanted)
			}
		})
	}
}

func TestClient_LastUpdatedAt(t *testing.T) {
	const (
		mockApp     = "mockApp"
		mockEnv     = "mockEnv"
		mockSvc     = "mockSvc"
		mockSvcARN  = "arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService"
		mockCluster = "mockCluster"
		mockService = "mockService"
	)
	mockError := errors.New("some error")
	mockTime := time.Unix(1494505756, 0)
	mockBeforeTime := time.Unix(1494505750, 0)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError error
		wanted      time.Time
	}{
		"error if fail to describe ECS service": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().Service(mockCluster, mockService).Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("get ECS service mockService: some error"),
		},
		"succeed": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().Service(mockCluster, mockService).Return(&ecs.Service{
						Deployments: []*awsecs.Deployment{
							{
								UpdatedAt: &mockTime,
							},
							{
								UpdatedAt: &mockBeforeTime,
							},
						},
					}, nil),
				)
			},
			wanted: mockTime,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mockECSClient := mocks.NewMockecsClient(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
				ecsClient:      mockECSClient,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter:  mockRgGetter,
				ecsClient: mockECSClient,
			}

			// WHEN
			get, err := client.LastUpdatedAt(mockApp, mockEnv, mockSvc)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, test.wanted)
			}
		})
	}
}

func TestClient_ForceUpdateService(t *testing.T) {
	const (
		mockApp     = "mockApp"
		mockEnv     = "mockEnv"
		mockSvc     = "mockSvc"
		mockSvcARN  = "arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService"
		mockCluster = "mockCluster"
		mockService = "mockService"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError error
	}{
		"return error if failed to update service": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().UpdateService(mockCluster, mockService, gomock.Any()).Return(errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().UpdateService(mockCluster, mockService, gomock.Any()).Return(nil),
				)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mockECSClient := mocks.NewMockecsClient(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
				ecsClient:      mockECSClient,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter:  mockRgGetter,
				ecsClient: mockECSClient,
			}

			// WHEN
			err := client.ForceUpdateService(mockApp, mockEnv, mockSvc)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_listActiveCopilotTasks(t *testing.T) {
	const (
		mockCluster   = "mockCluster"
		mockTaskGroup = "mockTaskGroup"
	)
	testError := errors.New("some error")

	tests := map[string]struct {
		inTaskGroup string
		inTaskID    string
		inOneOff    bool
		setupMocks  func(mocks clientMocks)

		wantedError error
		wanted      []*ecs.Task
	}{
		"errors if fail to list running tasks in a family": {
			inTaskGroup: mockTaskGroup,
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.ecsClient.EXPECT().RunningTasksInFamily(mockCluster, "mockTaskGroup").
						Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("list running tasks in family mockTaskGroup and cluster mockCluster: some error"),
		},
		"errors if fail to list running tasks": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.ecsClient.EXPECT().RunningTasks(mockCluster).
						Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("list running tasks in cluster mockCluster: some error"),
		},
		"success": {
			inTaskID: "123456",
			inOneOff: true,
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.ecsClient.EXPECT().RunningTasks(mockCluster).
						Return([]*ecs.Task{
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
								Tags: []*awsecs.Tag{
									{Key: aws.String("copilot-task")},
								},
							},
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
								Tags: []*awsecs.Tag{
									{Key: aws.String("copilot-application")},
								},
							},
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456788"),
							},
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/987765654"),
								Tags: []*awsecs.Tag{
									{Key: aws.String("copilot-task")},
								},
							},
						}, nil),
				)
			},
			wanted: []*ecs.Task{
				{
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
					Tags: []*awsecs.Tag{
						{Key: aws.String("copilot-task")},
					},
				},
			},
		},
		"success with oneOff disabled": {
			inTaskID: "123456",
			inOneOff: false,
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.ecsClient.EXPECT().RunningTasks(mockCluster).
						Return([]*ecs.Task{
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
								Tags: []*awsecs.Tag{
									{Key: aws.String("copilot-task")},
								},
							},
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
								Tags: []*awsecs.Tag{
									{Key: aws.String("copilot-application")},
								},
							},
						}, nil),
				)
			},
			wanted: []*ecs.Task{
				{
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
					Tags: []*awsecs.Tag{
						{Key: aws.String("copilot-task")},
					},
				},
				{
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/123456789"),
					Tags: []*awsecs.Tag{
						{Key: aws.String("copilot-application")},
					},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockECSTasksGetter := mocks.NewMockecsClient(ctrl)
			mocks := clientMocks{
				ecsClient: mockECSTasksGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				ecsClient: mockECSTasksGetter,
			}

			// WHEN
			got, err := client.listActiveCopilotTasks(listActiveCopilotTasksOpts{
				Cluster: mockCluster,
				ListTasksFilter: ListTasksFilter{
					TaskGroup:   test.inTaskGroup,
					TaskID:      test.inTaskID,
					CopilotOnly: test.inOneOff,
				},
			})

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, got, test.wanted)
			}
		})
	}
}

func TestClient_StopWorkloadTasks(t *testing.T) {
	mockCluster := "arn:aws::ecs:cluster/abcd1234"
	mockResource := resourcegroups.Resource{
		ARN: mockCluster,
	}
	mockECSTask := []*ecs.Task{
		{
			TaskArn: aws.String("deadbeef"),
			Tags: []*awsecs.Tag{
				{
					Key: aws.String("copilot-service"),
				},
			},
		},
		{
			TaskArn: aws.String("abcd"),
			Tags: []*awsecs.Tag{
				{
					Key: aws.String("copilot-service"),
				},
			},
		},
	}
	testCases := map[string]struct {
		inApp  string
		inEnv  string
		inTask string

		mockECS func(m *mocks.MockecsClient)
		mockrg  func(m *mocks.MockresourceGetter)

		wantErr error
	}{
		"success": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "service",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "phonetool-pdx-service").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef", "abcd"}, gomock.Any()).Return(nil)
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
		},
		"no tasks": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "service",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "phonetool-pdx-service").Return([]*ecs.Task{}, nil)
				m.EXPECT().StopTasks([]string{}, gomock.Any()).Return(nil)
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
		},
		"failure getting resources": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "service",

			mockECS: func(m *mocks.MockecsClient) {},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("get cluster resources for environment pdx: some error"),
		},
		"failure stopping tasks": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "service",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "phonetool-pdx-service").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef", "abcd"}, gomock.Any()).Return(errors.New("some error"))
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
			wantErr: errors.New("some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECS := mocks.NewMockecsClient(ctrl)
			mockrg := mocks.NewMockresourceGetter(ctrl)

			tc.mockECS(mockECS)
			tc.mockrg(mockrg)

			c := Client{
				ecsClient: mockECS,
				rgGetter:  mockrg,
			}

			// WHEN
			err := c.StopWorkloadTasks(tc.inApp, tc.inEnv, tc.inTask)

			// THEN
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_StopOneOffTasks(t *testing.T) {
	mockCluster := "arn:aws::ecs:cluster/abcd1234"
	mockResource := resourcegroups.Resource{
		ARN: mockCluster,
	}
	mockECSTask := []*ecs.Task{
		{
			TaskArn: aws.String("deadbeef"),
			Tags: []*awsecs.Tag{
				{
					Key: aws.String("copilot-task"),
				},
			},
		},
	}
	testCases := map[string]struct {
		inApp  string
		inEnv  string
		inTask string

		mockECS func(m *mocks.MockecsClient)
		mockrg  func(m *mocks.MockresourceGetter)

		wantErr error
	}{
		"success": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "copilot-cooltask").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef"}, gomock.Any()).Return(nil)
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
		},
		"no tasks": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "copilot-cooltask").Return([]*ecs.Task{}, nil)
				m.EXPECT().StopTasks([]string{}, gomock.Any()).Return(nil)
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
		},
		"failure getting resources": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("get cluster resources for environment pdx: some error"),
		},
		"failure stopping tasks": {
			inApp:  "phonetool",
			inEnv:  "pdx",
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().RunningTasksInFamily(mockCluster, "copilot-cooltask").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef"}, gomock.Any()).Return(errors.New("some error"))
			},
			mockrg: func(m *mocks.MockresourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, map[string]string{
					"copilot-application": "phonetool",
					"copilot-environment": "pdx",
				}).Return([]*resourcegroups.Resource{&mockResource}, nil).Times(2)
			},
			wantErr: errors.New("some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECS := mocks.NewMockecsClient(ctrl)
			mockrg := mocks.NewMockresourceGetter(ctrl)

			tc.mockECS(mockECS)
			tc.mockrg(mockrg)

			c := Client{
				ecsClient: mockECS,
				rgGetter:  mockrg,
			}

			// WHEN
			err := c.StopOneOffTasks(tc.inApp, tc.inEnv, tc.inTask)

			// THEN
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_StopDefaultClusterTasks(t *testing.T) {
	mockECSTask := []*ecs.Task{
		{
			TaskArn: aws.String("deadbeef"),
			Tags: []*awsecs.Tag{
				{
					Key: aws.String("copilot-task"),
				},
			},
		},
		{
			TaskArn: aws.String("deadbeef"),
			Tags: []*awsecs.Tag{
				{
					Key: aws.String("copilot-service"),
				},
			},
		},
	}
	testCases := map[string]struct {
		inTask string

		mockECS func(m *mocks.MockecsClient)

		wantErr error
	}{
		"success": {

			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().DefaultCluster().Return("cluster", nil)
				m.EXPECT().RunningTasksInFamily(gomock.Any(), "copilot-cooltask").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef"}, gomock.Any()).Return(nil)
			},
		},
		"failure stopping tasks": {
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().DefaultCluster().Return("cluster", nil)
				m.EXPECT().RunningTasksInFamily(gomock.Any(), "copilot-cooltask").Return(mockECSTask, nil)
				m.EXPECT().StopTasks([]string{"deadbeef"}, gomock.Any()).Return(errors.New("some error"))
			},
			wantErr: errors.New("some error"),
		},
		"failure getting cluster": {
			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().DefaultCluster().Return("", errors.New("some error"))
			},
			wantErr: errors.New("get default cluster: some error"),
		},
		"failure listing tasks": {
			inTask: "cooltask",

			mockECS: func(m *mocks.MockecsClient) {
				m.EXPECT().DefaultCluster().Return("cluster", nil)
				m.EXPECT().RunningTasksInFamily(gomock.Any(), "copilot-cooltask").Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("list running tasks in family copilot-cooltask and cluster cluster: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECS := mocks.NewMockecsClient(ctrl)

			tc.mockECS(mockECS)

			c := Client{
				ecsClient: mockECS,
			}

			// WHEN
			err := c.StopDefaultClusterTasks(tc.inTask)

			// THEN
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
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
		setupMocks func(m *mocks.MockecsClient)

		wantedTaskDefinition *ecs.TaskDefinition
		wantedError          error
	}{
		"unable to retrieve task definition": {
			setupMocks: func(m *mocks.MockecsClient) {
				m.EXPECT().TaskDefinition("phonetool-test-svc").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get task definition phonetool-test-svc of service svc: some error"),
		},
		"successfully return task definition information": {
			setupMocks: func(m *mocks.MockecsClient) {
				m.EXPECT().TaskDefinition("phonetool-test-svc").Return(&ecs.TaskDefinition{
					ExecutionRoleArn: aws.String("execution-role"),
					TaskRoleArn:      aws.String("task-role"),
					ContainerDefinitions: []*awsecs.ContainerDefinition{
						{
							Name:  aws.String("the-container"),
							Image: aws.String("beautiful-image"),
							Environment: []*awsecs.KeyValuePair{
								{
									Name:  aws.String("weather"),
									Value: aws.String("snowy"),
								},
								{
									Name:  aws.String("temperature"),
									Value: aws.String("low"),
								},
							},
							Secrets: []*awsecs.Secret{
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
			wantedTaskDefinition: &ecs.TaskDefinition{
				ExecutionRoleArn: aws.String("execution-role"),
				TaskRoleArn:      aws.String("task-role"),
				ContainerDefinitions: []*awsecs.ContainerDefinition{
					{
						Name:  aws.String("the-container"),
						Image: aws.String("beautiful-image"),
						Environment: []*awsecs.KeyValuePair{
							{
								Name:  aws.String("weather"),
								Value: aws.String("snowy"),
							},
							{
								Name:  aws.String("temperature"),
								Value: aws.String("low"),
							},
						},
						Secrets: []*awsecs.Secret{
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
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECS := mocks.NewMockecsClient(ctrl)
			tc.setupMocks(mockECS)

			c := Client{
				ecsClient: mockECS,
			}

			// WHEN
			got, err := c.TaskDefinition(testApp, testEnv, testSvc)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTaskDefinition, got)
			}
		})
	}
}

func Test_NetworkConfiguration(t *testing.T) {
	const (
		testApp = "phonetool"
		testSvc = "svc"
		testEnv = "test"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey: testApp,
		deploy.EnvTagKey: testEnv,
	}

	testCases := map[string]struct {
		setupMocks func(m clientMocks)

		wantedNetworkConfig *ecs.NetworkConfiguration
		wantedError         error
	}{
		"errors if fail to get resources by tags": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("get cluster resources for environment test: some error"),
		},
		"errors if no cluster found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{}, nil),
				)
			},
			wantedError: fmt.Errorf("no cluster found in environment test"),
		},
		"errors if more than one cluster found": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockARN1"}, {ARN: "mockARN2"},
						}, nil),
				)
			},
			wantedError: fmt.Errorf("more than one cluster is found in environment test"),
		},
		"successfully retrieve network configuration": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "cluster-1"},
						}, nil),
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, map[string]string{
						deploy.AppTagKey:     testApp,
						deploy.EnvTagKey:     testEnv,
						deploy.ServiceTagKey: testSvc,
					}).Return([]*resourcegroups.Resource{
						{ARN: "arn:aws:ecs:us-west-2:1234567890:service/my-project-test-Cluster-9F7Y0RLP60R7/my-project-test-myService-JSOH5GYBFAIB"},
					}, nil),
					m.ecsClient.EXPECT().NetworkConfiguration("cluster-1", "my-project-test-myService-JSOH5GYBFAIB").Return(&ecs.NetworkConfiguration{
						AssignPublicIp: "1.2.3.4",
						SecurityGroups: []string{"sg-1", "sg-2"},
						Subnets:        []string{"sn-1", "sn-2"},
					}, nil),
				)

			},
			wantedNetworkConfig: &ecs.NetworkConfiguration{
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

			// GIVEN
			m := clientMocks{
				resourceGetter: mocks.NewMockresourceGetter(ctrl),
				ecsClient:      mocks.NewMockecsClient(ctrl),
			}

			tc.setupMocks(m)

			client := Client{
				rgGetter:  m.resourceGetter,
				ecsClient: m.ecsClient,
			}

			// WHEN
			get, err := client.NetworkConfiguration(testApp, testEnv, testSvc)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, tc.wantedNetworkConfig)
			}
		})
	}
}

func Test_NetworkConfigurationForJob(t *testing.T) {
	const (
		testApp = "testApp"
		testEnv = "testEnv"
		testJob = "testJob"
		testARN = "arn:aws:states:us-east-1:1234456789012:stateMachine:testApp-testEnv-testJob"

		testStateMachineDefinition = `{
          "Version": "1.0",
          "Comment": "Run AWS Fargate task",
          "StartAt": "Run Fargate Task",
          "States": {
            "Run Fargate Task": {
              "Type": "Task",
              "Resource": "arn:aws:states:::ecs:runTask.sync",
              "Parameters": {
                "LaunchType": "FARGATE",
                "PlatformVersion": "LATEST",
                "Cluster": "cluster",
                "TaskDefinition": "def",
                "PropagateTags": "TASK_DEFINITION",
                "Group.$": "$$.Execution.Name",
                "NetworkConfiguration": {
                  "AwsvpcConfiguration": {
                    "Subnets": ["sbn-1", "sbn-2"],
                    "AssignPublicIp": "ENABLED",
                    "SecurityGroups": ["sg-1", "sg-2"]
                  }
                }
              },
              "End": true
            }
          }
		}`
	)

	testCases := map[string]struct {
		setupMocks func(m clientMocks)

		wantedConfig *ecs.NetworkConfiguration
		wantedError  error
	}{
		"success": {
			setupMocks: func(m clientMocks) {
				m.resourceGetter.EXPECT().GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, map[string]string{
					deploy.AppTagKey:     testApp,
					deploy.EnvTagKey:     testEnv,
					deploy.ServiceTagKey: testJob,
				}).Return([]*resourcegroups.Resource{
					{
						ARN: "random-arn-doesn't matter",
					},
					{
						ARN: testARN,
					},
				}, nil)

				m.StepFuncClient.EXPECT().StateMachineDefinition(testARN).Return(testStateMachineDefinition, nil)
			},
			wantedConfig: &ecs.NetworkConfiguration{
				Subnets:        []string{"sbn-1", "sbn-2"},
				SecurityGroups: []string{"sg-1", "sg-2"},
				AssignPublicIp: "ENABLED",
			},
		},
		"fail to get resources by tags": {
			setupMocks: func(m clientMocks) {
				m.resourceGetter.EXPECT().GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, map[string]string{
					deploy.AppTagKey:     testApp,
					deploy.EnvTagKey:     testEnv,
					deploy.ServiceTagKey: testJob,
				}).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get state machine resource by tags for job testJob: some error"),
		},
		"state machine resource not found": {
			setupMocks: func(m clientMocks) {
				m.resourceGetter.EXPECT().GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, map[string]string{
					deploy.AppTagKey:     testApp,
					deploy.EnvTagKey:     testEnv,
					deploy.ServiceTagKey: testJob,
				}).Return([]*resourcegroups.Resource{
					{
						ARN: "rabbit",
					},
					{
						ARN: "cabbage",
					},
				}, nil)
			},
			wantedError: errors.New("state machine for job testJob not found"),
		},
		"fail to get state machine definition": {
			setupMocks: func(m clientMocks) {
				m.resourceGetter.EXPECT().GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, map[string]string{
					deploy.AppTagKey:     testApp,
					deploy.EnvTagKey:     testEnv,
					deploy.ServiceTagKey: testJob,
				}).Return([]*resourcegroups.Resource{
					{
						ARN: "random-arn-doesn't matter",
					},
					{
						ARN: testARN,
					},
				}, nil)

				m.StepFuncClient.EXPECT().StateMachineDefinition(testARN).Return("", errors.New("some error"))
			},
			wantedError: errors.New("get state machine definition for job testJob: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			m := clientMocks{
				StepFuncClient: mocks.NewMockstepFunctionsClient(ctrl),
				resourceGetter: mocks.NewMockresourceGetter(ctrl),
			}
			tc.setupMocks(m)

			client := Client{
				rgGetter:       m.resourceGetter,
				StepFuncClient: m.StepFuncClient,
			}

			// WHEN
			get, err := client.NetworkConfigurationForJob(testApp, testEnv, testJob)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, tc.wantedConfig)
			}
		})
	}
}

func Test_HasNonZeroExitCode(t *testing.T) {
	testCases := map[string]struct {
		inTaskARNs  []string
		inGroupName string
		inCluster   string
		setupMocks  func(m clientMocks)

		wantedError error
	}{

		"returns the non zero exit code of the essential container": {
			inCluster:  "cluster-1",
			inTaskARNs: []string{"mockTask1"},
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.ecsClient.EXPECT().DescribeTasks("cluster-1", []string{"mockTask1"}).Return([]*ecs.Task{
						{
							TaskArn:           aws.String("arn:aws:ecs:us-west-2:123456789:task/4082490ee6c245e09d2145010aa1ba8d"),
							TaskDefinitionArn: aws.String("arn:aws:ecs:us-west-2:1233454566:task-definition/CdkExampleStacknametaskdefinitionCA96DCAA:1"),
							StoppedReason:     aws.String("Task failed to start"),
							LastStatus:        aws.String("STOPPED"),
							Containers: []*awsecs.Container{
								{
									Name:     aws.String("the-one-and-only-one-container"),
									ExitCode: aws.Int64(1),
								},
							},
						},
					}, nil),
					m.ecsClient.EXPECT().TaskDefinition("arn:aws:ecs:us-west-2:1233454566:task-definition/CdkExampleStacknametaskdefinitionCA96DCAA:1").Return(&ecs.TaskDefinition{
						ExecutionRoleArn: aws.String("execution-role"),
						TaskRoleArn:      aws.String("task-role"),
						ContainerDefinitions: []*awsecs.ContainerDefinition{
							{
								Name:       aws.String("the-one-and-only-one-container"),
								Image:      aws.String("beautiful-image"),
								EntryPoint: aws.StringSlice([]string{"enter", "here"}),
								Command:    aws.StringSlice([]string{"do", "not", "enter", "here"}),
								Essential:  aws.Bool(true),
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
					}, nil),
				)
			},
			wantedError: fmt.Errorf("container the-one-and-only-one-container in task 4082490ee6c245e09d2145010aa1ba8d exited with status code 1"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			m := clientMocks{
				ecsClient: mocks.NewMockecsClient(ctrl),
			}

			tc.setupMocks(m)

			client := Client{
				ecsClient: m.ecsClient,
			}

			// WHEN
			err := client.HasNonZeroExitCode(tc.inTaskARNs, tc.inCluster)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}
