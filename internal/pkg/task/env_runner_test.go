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
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvRunner_Run(t *testing.T) {
	inApp := "my-app"
	inEnv := "my-env"

	filtersForSubnetID := []ec2.Filter{
		{
			Name:   fmtTagFilterForEnv,
			Values: []string{inEnv},
		},
		{
			Name:   fmtTagFilterForApp,
			Values: []string{inApp},
		},
	}
	filtersForSecurityGroup := append(filtersForSubnetID, ec2.Filter{
		Name:   "tag:aws:cloudformation:logical-id",
		Values: []string{"EnvironmentSecurityGroup"},
	})

	mockClusterGetter := func(m *mocks.MockClusterGetter) {
		m.EXPECT().ClusterARN(inApp, inEnv).Return("cluster-1", nil)
	}
	mockVPCGetterAny := func(m *mocks.MockVPCGetter) {
		m.EXPECT().SubnetIDs(gomock.Any()).AnyTimes()
		m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
	}
	mockStarterNotRun := func(m *mocks.MockRunner) {
		m.EXPECT().RunTask(gomock.Any()).Times(0)
	}
	mockEnvironmentDescriberAny := func(m *mocks.MockenvironmentDescriber) {
		m.EXPECT().Describe().AnyTimes()
	}
	mockEnvironmentDescriberValid := func(m *mocks.MockenvironmentDescriber) {
		m.EXPECT().Describe().Return(&describe.EnvDescription{
			EnvironmentVPC: describe.EnvironmentVPC{
				ID:               "vpc-012abcd345",
				PublicSubnetIDs:  []string{"subnet-0789ab", "subnet-0123cd"},
				PrivateSubnetIDs: []string{"subnet-023ff", "subnet-04af"},
			},
		}, nil)
	}

	taskWithENI = ecs.Task{
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
	taskWithNoENI = ecs.Task{
		TaskArn: aws.String("task-2"),
	}

	testCases := map[string]struct {
		count          int
		groupName      string
		os             string
		arch           string
		securityGroups []string

		MockVPCGetter            func(m *mocks.MockVPCGetter)
		MockClusterGetter        func(m *mocks.MockClusterGetter)
		mockStarter              func(m *mocks.MockRunner)
		mockEnvironmentDescriber func(m *mocks.MockenvironmentDescriber)

		wantedError error
		wantedTasks []*Task
	}{
		"failed to get cluster": {
			MockClusterGetter: func(m *mocks.MockClusterGetter) {
				m.EXPECT().ClusterARN(inApp, inEnv).Return("", errors.New("error getting resources"))
			},
			MockVPCGetter:            mockVPCGetterAny,
			mockStarter:              mockStarterNotRun,
			mockEnvironmentDescriber: mockEnvironmentDescriberAny,
			wantedError:              fmt.Errorf("get cluster for environment %s: %w", inEnv, errors.New("error getting resources")),
		},
		"failed to get env description": {
			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			mockEnvironmentDescriber: func(m *mocks.MockenvironmentDescriber) {
				m.EXPECT().Describe().Return(nil, errors.New("error getting env description"))
			},
			wantedError: fmt.Errorf(fmtErrDescribeEnvironment, inEnv, errors.New("error getting env description")),
		},
		"no subnet is found": {
			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			mockEnvironmentDescriber: func(m *mocks.MockenvironmentDescriber) {
				m.EXPECT().Describe().Return(&describe.EnvDescription{
					EnvironmentVPC: describe.EnvironmentVPC{
						ID:               "vpc-012abcd345",
						PublicSubnetIDs:  []string{},
						PrivateSubnetIDs: []string{"subnet-023ff", "subnet-04af"},
					},
				}, nil)
			},
			wantedError: errNoSubnetFound,
		},
		"failed to get security groups": {
			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).
					Return(nil, errors.New("error getting security groups"))
			},
			mockStarter:              mockStarterNotRun,
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedError:              fmt.Errorf(fmtErrSecurityGroupsFromEnv, inEnv, errors.New("error getting security groups")),
		},
		"failed with too many security groups": {
			securityGroups: []string{"sg-2", "sg-3", "sg-4", "sg-5", "sg-6"},

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter:              mockStarterNotRun,
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedError:              fmt.Errorf(fmtErrNumSecurityGroups, 6, "sg-1,sg-2,sg-3,sg-4,sg-5,sg-6"),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return(nil, errors.New("error running task"))
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedError: &errRunTask{
				groupName: "my-task",
				parentErr: errors.New("error running task"),
			},
		},
		"run in env success": {
			count:     1,
			groupName: "my-task",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"run in env with extra security groups success": {
			count:          1,
			groupName:      "my-task",
			securityGroups: []string{"sg-1", "sg-extra"},

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2", "sg-extra"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "LATEST",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"run in env with windows os success 2019 core": {
			count:     1,
			groupName: "my-task",
			os:        "WINDOWS_SERVER_2019_CORE",
			arch:      "X86_64",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"run in env with windows os success 2019 full": {
			count:     1,
			groupName: "my-task",
			os:        "WINDOWS_SERVER_2019_FULL",
			arch:      "X86_64",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"run in env with windows os success 2022 core": {
			count:     1,
			groupName: "my-task",
			os:        "WINDOWS_SERVER_2022_CORE",
			arch:      "X86_64",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
			wantedTasks: []*Task{
				{
					TaskARN: "task-1",
					ENI:     "eni-1",
				},
			},
		},
		"run in env with windows os success 2022 full": {
			count:     1,
			groupName: "my-task",
			os:        "WINDOWS_SERVER_2022_FULL",
			arch:      "X86_64",

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
					SecurityGroups:  []string{"sg-1", "sg-2"},
					TaskFamilyName:  taskFamilyName("my-task"),
					StartedBy:       startedBy,
					PlatformVersion: "1.0.0",
					EnableExec:      true,
				}).Return([]*ecs.Task{&taskWithENI}, nil)
			},
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
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

			MockClusterGetter: mockClusterGetter,
			MockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SecurityGroups(filtersForSecurityGroup).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockRunner) {
				m.EXPECT().RunTask(ecs.RunTaskInput{
					Cluster:         "cluster-1",
					Count:           1,
					Subnets:         []string{"subnet-0789ab", "subnet-0123cd"},
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
			mockEnvironmentDescriber: mockEnvironmentDescriberValid,
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockVPCGetter := mocks.NewMockVPCGetter(ctrl)
			MockClusterGetter := mocks.NewMockClusterGetter(ctrl)
			mockStarter := mocks.NewMockRunner(ctrl)
			mockEnvironmentDescriber := mocks.NewMockenvironmentDescriber(ctrl)

			tc.MockVPCGetter(MockVPCGetter)
			tc.MockClusterGetter(MockClusterGetter)
			tc.mockStarter(mockStarter)
			tc.mockEnvironmentDescriber(mockEnvironmentDescriber)

			task := &EnvRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				App: inApp,
				Env: inEnv,

				OS: tc.os,

				SecurityGroups: tc.securityGroups,

				VPCGetter:            MockVPCGetter,
				ClusterGetter:        MockClusterGetter,
				Starter:              mockStarter,
				EnvironmentDescriber: mockEnvironmentDescriber,
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
