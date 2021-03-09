// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvRunner_Run(t *testing.T) {
	inApp := "my-app"
	inEnv := "my-env"

	filtersForSubnetID := []ec2.Filter{
		{
			Name:   tagFilterNameForEnv,
			Values: []string{inEnv},
		},
		{
			Name:   tagFilterNameForApp,
			Values: []string{inApp},
		},
	}
	filtersForSecurityGroup := append(filtersForSubnetID, ec2.Filter{
		Name:   "tag:aws:cloudformation:logical-id",
		Values: []string{"EnvironmentSecurityGroup"},
	})

	MockClusterGetter := func(m *mocks.MockClusterGetter) {
		m.EXPECT().ClusterARN(inApp, inEnv).Return("cluster-1", nil)
	}
	MockVPCGetterAny := func(m *mocks.MockVPCGetter) {
		m.EXPECT().SubnetIDs(gomock.Any()).AnyTimes()
		m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
	}
	mockStarterNotRun := func(m *mocks.MockRunner) {
		m.EXPECT().RunTask(gomock.Any()).Times(0)
	}

	testCases := map[string]struct {
		count     int
		groupName string

		MockVPCGetter     func(m *mocks.MockVPCGetter)
		MockClusterGetter func(m *mocks.MockClusterGetter)
		mockStarter       func(m *mocks.MockRunner)

		wantedError error
		wantedTasks []*Task
	}{
		"failed to get cluster": {
			MockClusterGetter: func(m *mocks.MockClusterGetter) {
				m.EXPECT().ClusterARN(inApp, inEnv).Return("", errors.New("error getting resources"))
			},
			MockVPCGetter: MockVPCGetterAny,
			mockStarter:   mockStarterNotRun,
			wantedError:   fmt.Errorf("get cluster for environment %s: %w", inEnv, errors.New("error getting resources")),
		},
		"failed to get subnets": {
			MockClusterGetter: MockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForSubnetID).
					Return(nil, errors.New("error getting subnets"))
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			wantedError: fmt.Errorf(fmtErrPublicSubnetsFromEnv, inEnv, errors.New("error getting subnets")),
		},
		"no subnet is found": {
			MockClusterGetter: MockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForSubnetID).
					Return([]string{}, nil)
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			wantedError: ErrNoSubnetFound,
		},
		"failed to get security groups": {
			MockClusterGetter: MockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(gomock.Any()).Return([]string{"subnet-1"}, nil)
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).
					Return(nil, errors.New("error getting security groups"))
			},
			mockStarter: mockStarterNotRun,
			wantedError: fmt.Errorf(fmtErrSecurityGroupsFromEnv, inEnv, errors.New("error getting security groups")),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",

			MockClusterGetter: MockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForSubnetID).Return([]string{"subnet-1", "subnet-2"}, nil)
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:        "cluster-1",
					Count:          1,
					Subnets:        []string{"subnet-1", "subnet-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
					TaskFamilyName: taskFamilyName("my-task"),
					StartedBy:      startedBy,
				}).Return(nil, errors.New("error running task"))
			},

			wantedError: &errRunTask{
				groupName: "my-task",
				parentErr: errors.New("error running task"),
			},
		},
		"run in env success": {
			count:     1,
			groupName: "my-task",

			MockClusterGetter: MockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForSubnetID).Return([]string{"subnet-1", "subnet-2"}, nil)
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:        "cluster-1",
					Count:          1,
					Subnets:        []string{"subnet-1", "subnet-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
					TaskFamilyName: taskFamilyName("my-task"),
					StartedBy:      startedBy,
				}).Return([]*ecs.Task{
					{
						TaskArn: aws.String("task-1"),
					},
				}, nil)
			},
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockVPCGetter := mocks.NewMockVPCGetter(ctrl)
			MockClusterGetter := mocks.NewMockClusterGetter(ctrl)
			mockStarter := mocks.NewMockRunner(ctrl)

			tc.MockVPCGetter(MockVPCGetter)
			tc.MockClusterGetter(MockClusterGetter)
			tc.mockStarter(mockStarter)

			task := &EnvRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				App: inApp,
				Env: inEnv,

				VPCGetter:     MockVPCGetter,
				ClusterGetter: MockClusterGetter,
				Starter:       mockStarter,
			}

			tasks, err := task.Run()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTasks, tasks)
			}
		})
	}
}
