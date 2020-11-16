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
	ecsTaskGetter  *mocks.MockfamilyRunningTasksGetter
}

func TestClient_Cluster(t *testing.T) {
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
			get, err := client.Cluster(mockApp, mockEnv)

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
					m.ecsTaskGetter.EXPECT().FamilyRunningTasks("mockCluster", "mockApp-mockEnv-mockWl").Return(nil, testError),
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
					m.ecsTaskGetter.EXPECT().FamilyRunningTasks("mockCluster", "mockApp-mockEnv-mockWl").Return([]*ecs.Task{
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
			mockECSTasksGetter := mocks.NewMockfamilyRunningTasksGetter(ctrl)
			mockNewECSTasksGetter := func() familyRunningTasksGetter {
				return mockECSTasksGetter
			}
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
				ecsTaskGetter:  mockECSTasksGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter:          mockRgGetter,
				newECSTasksGetter: mockNewECSTasksGetter,
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
