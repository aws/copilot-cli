// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNetworkConfigRunner_Run(t *testing.T) {
	testCases := map[string]struct {
		count     int
		groupName string

		subnets        []string
		securityGroups []string

		mockClusterGetter func(m *mocks.MockDefaultClusterGetter)
		mockStarter       func(m *mocks.MockTaskRunner)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get clusters": {
			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("", errors.New("error getting default cluster"))
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf(fmtErrDefaultCluster, errors.New("error getting default cluster")),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",

			subnets:        []string{"subnet-1", "subnet-2"},
			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Return(nil, errors.New("error running task"))
			},

			wantedError: &errRunTask{
				groupName: "my-task",
				parentErr: errors.New("error running task"),
			},
		},
		"successfully kick off task with both input subnets and security groups": {
			count:     1,
			groupName: "my-task",

			subnets:        []string{"subnet-1", "subnet-2"},
			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:        "cluster-1",
					Count:          1,
					Subnets:        []string{"subnet-1", "subnet-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
					TaskFamilyName: taskFamilyName("my-task"),
					StartedBy:      startedBy,
				}).Return([]string{"task-1"}, nil)
			},

			wantedARNs: []string{"task-1"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClusterGetter := mocks.NewMockDefaultClusterGetter(ctrl)
			mockStarter := mocks.NewMockTaskRunner(ctrl)

			tc.mockClusterGetter(mockClusterGetter)
			tc.mockStarter(mockStarter)

			task := &NetworkConfigRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				Subnets:        tc.subnets,
				SecurityGroups: tc.securityGroups,

				ClusterGetter: mockClusterGetter,
				Starter:       mockStarter,
			}

			arns, err := task.Run()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}
