// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"testing"

	awsecs "github.com/aws/aws-sdk-go/service/ecs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	attachmentTypeName = "ElasticNetworkInterface"
	detailsKeyName     = "networkInterfaceId"
)

var taskWithENI = ecs.Task{
	TaskArn: aws.String("task-1"),
	Attachments: []*awsecs.Attachment{
		{
			Type: aws.String(attachmentTypeName),
			Details: []*awsecs.KeyValuePair{
				{
					Name:  aws.String(detailsKeyName),
					Value: aws.String("eni-1"),
				},
			},
		},
	},
}

var taskWithNoENI = ecs.Task{
	TaskArn: aws.String("task-2"),
}

func TestNetworkConfigRunner_Run(t *testing.T) {
	testCases := map[string]struct {
		count     int
		groupName string

		cluster        string
		subnets        []string
		securityGroups []string

		os   string
		arch string

		mockClusterGetter func(m *mocks.MockDefaultClusterGetter)
		mockStarter       func(m *mocks.MockRunner)
		MockVPCGetter     func(m *mocks.MockVPCGetter)

		wantedError error
		wantedTasks []*Task
	}{
		"failed to get default cluster": {
			subnets: []string{"subnet-1", "subnet-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("", errors.New("error getting default cluster"))
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs().AnyTimes()
				m.EXPECT().SecurityGroups().AnyTimes()
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: &errGetDefaultCluster{
				parentErr: errors.New("error getting default cluster"),
			},
		},
		"failed to kick off tasks with input subnets and security groups": {
			count:     1,
			groupName: "my-task",

			subnets:        []string{"subnet-1", "subnet-2"},
			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Times(0)
			},
			mockStarter: func(m *mocks.MockRunner) {
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
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Times(0)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-1", "subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"failed to get default subnets": {
			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().AnyTimes()
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Return(nil, errors.New("error getting subnets"))
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf(fmtErrDefaultSubnets, errors.New("error getting subnets")),
		},
		"successfully kick off task with default subnets": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"eni information not found for several tasks": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{
					&taskWithENI,
					&taskWithNoENI,
					&taskWithNoENI,
				}, nil)
			},
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
				{
					TaskARN: "task-2",
				},
				{
					TaskARN: "task-2",
				},
			},
		},
		"successfully kick off task with specified cluster": {
			count:     1,
			groupName: "my-task",

			cluster:        "special-cluster",
			subnets:        []string{"subnet-1", "subnet-2"},
			securityGroups: []string{"sg-1", "sg-2"},

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Times(0)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Times(0)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "special-cluster",
					Count:           1,
					Subnets:         []string{"subnet-1", "subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"successfully kick off task with platform version for windows 2019 core": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			os:   "WINDOWS_SERVER_2019_CORE",
			arch: "X86_64",

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"successfully kick off task with platform version for windows 2019 full": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			os:   "WINDOWS_SERVER_2019_FULL",
			arch: "X86_64",

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"successfully kick off task with platform version for windows 2022 core": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			os:   "WINDOWS_SERVER_2022_CORE",
			arch: "X86_64",

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"successfully kick off task with platform version for windows 2022 full": {
			count:     1,
			groupName: "my-task",

			securityGroups: []string{"sg-1", "sg-2"},

			os:   "WINDOWS_SERVER_2022_FULL",
			arch: "X86_64",

			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).
					Return([]string{"default-subnet-1", "default-subnet-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"default-subnet-1", "default-subnet-2"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},

			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockVPCGetter := mocks.NewMockVPCGetter(ctrl)
			mockClusterGetter := mocks.NewMockDefaultClusterGetter(ctrl)
			mockStarter := mocks.NewMockRunner(ctrl)

			tc.MockVPCGetter(MockVPCGetter)
			tc.mockClusterGetter(mockClusterGetter)
			tc.mockStarter(mockStarter)

			task := &ConfigRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				Cluster:        tc.cluster,
				Subnets:        tc.subnets,
				SecurityGroups: tc.securityGroups,

				VPCGetter:     MockVPCGetter,
				ClusterGetter: mockClusterGetter,
				Starter:       mockStarter,

				OS: tc.os,
			}

			tasks, err := task.Run()
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTasks, tasks)
			}
		})
	}
}
