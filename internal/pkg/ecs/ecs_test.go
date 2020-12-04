// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
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

func TestClient_ServiceARN(t *testing.T) {
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
			get, err := client.ServiceARN(mockApp, mockEnv, mockSvc)

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
					m.ecsClient.EXPECT().ServiceTasks(mockCluster, mockService).Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("get tasks for service mockService: some error"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockSvcARN},
						}, nil),
					m.ecsClient.EXPECT().ServiceTasks(mockCluster, mockService).Return([]*ecs.Task{
						{TaskArn: aws.String("mockTaskARN")},
					}, nil),
				)
			},
			wanted: &ServiceDesc{
				ClusterName: mockCluster,
				Name:        mockService,
				Tasks: []*ecs.Task{
					{TaskArn: aws.String("mockTaskARN")},
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

func TestClient_ListActiveWorkloadTasks(t *testing.T) {
	const (
		mockApp = "mockApp"
		mockEnv = "mockEnv"
		mockWl  = "mockWl"
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
		wantedTasks   []string
	}{
		"errors if fail to get cluster": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("get cluster for env mockEnv: get cluster resources for environment mockEnv: some error"),
		},
		"errors if fail to get running tasks": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockCluster"},
						}, nil),
					m.ecsClient.EXPECT().RunningTasksInFamily("mockCluster", "mockApp-mockEnv-mockWl").Return(nil, testError),
				)
			},
			wantedError: fmt.Errorf("list tasks that belong to family mockApp-mockEnv-mockWl: some error"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(clusterResourceType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: "mockCluster"},
						}, nil),
					m.ecsClient.EXPECT().RunningTasksInFamily("mockCluster", "mockApp-mockEnv-mockWl").Return([]*ecs.Task{
						{
							TaskArn: aws.String("mockTask1"),
						},
						{
							TaskArn: aws.String("mockTask2"),
						},
					}, nil),
				)
			},
			wantedCluster: "mockCluster",
			wantedTasks:   []string{"mockTask1", "mockTask2"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mockECSTasksGetter := mocks.NewMockecsClient(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
				ecsClient:      mockECSTasksGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter:  mockRgGetter,
				ecsClient: mockECSTasksGetter,
			}

			// WHEN
			getCluster, getTasks, err := client.ListActiveWorkloadTasks(mockApp, mockEnv, mockWl)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, getCluster, test.wantedCluster)
				require.Equal(t, getTasks, test.wantedTasks)
			}
		})
	}
}
