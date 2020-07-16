// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEnvRunner_Run(t *testing.T) {
	inApp := "my-app"
	inEnv := "my-env"

	resourceTagFiltersForCluster := map[string]string{
		deploy.AppTagKey: inApp,
		deploy.EnvTagKey: inEnv,
	}
	filtersForVPCFromAppEnv := []ec2.Filter{
		ec2.Filter{
			Name:   tagFilterNameForEnv,
			Values: []string{inEnv},
		},
		ec2.Filter{
			Name:   tagFilterNameForApp,
			Values: []string{inApp},
		},
	}

	mockResourceGetterWithCluster := func(m *mocks.MockResourceGetter) {
		m.EXPECT().GetResourcesByTags(clusterResourceType, resourceTagFiltersForCluster).Return([]*resourcegroups.Resource{
			&resourcegroups.Resource{ARN: "cluster-1"},
		}, nil)
	}
	mockVPCGetterAny := func(m *mocks.MockVPCGetter) {
		m.EXPECT().SubnetIDs(gomock.Any()).AnyTimes()
		m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
	}
	mockStarterNotRun := func(m *mocks.MockTaskRunner) {
		m.EXPECT().RunTask(gomock.Any()).Times(0)
	}

	testCases := map[string]struct {
		count     int
		groupName string

		mockVPCGetter      func(m *mocks.MockVPCGetter)
		mockResourceGetter func(m *mocks.MockResourceGetter)
		mockStarter        func(m *mocks.MockTaskRunner)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get cluster": {
			mockResourceGetter: func(m *mocks.MockResourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, resourceTagFiltersForCluster).
					Return(nil, errors.New("error getting resources"))
			},
			mockVPCGetter: mockVPCGetterAny,
			mockStarter:   mockStarterNotRun,
			wantedError:   fmt.Errorf(fmtErrClusterFromEnv, inEnv, errors.New("error getting resources")),
		},
		"no cluster found": {
			mockResourceGetter: func(m *mocks.MockResourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, resourceTagFiltersForCluster).
					Return([]*resourcegroups.Resource{}, nil)
			},
			mockVPCGetter: mockVPCGetterAny,
			mockStarter:   mockStarterNotRun,
			wantedError:   fmt.Errorf("no cluster found in env %s", inEnv),
		},
		"more than one cluster is found": {
			mockResourceGetter: func(m *mocks.MockResourceGetter) {
				m.EXPECT().GetResourcesByTags(clusterResourceType, resourceTagFiltersForCluster).
					Return([]*resourcegroups.Resource{
						&resourcegroups.Resource{
							ARN: "cluster-1",
						},
						&resourcegroups.Resource{
							ARN: "cluster-2",
						},
				}, nil)
			},
			mockVPCGetter: mockVPCGetterAny,
			mockStarter:   mockStarterNotRun,
			wantedError:   fmt.Errorf(fmtErrMoreThanOneClusterFromEnv, inEnv),
		},
		"failed to get subnets": {
			mockResourceGetter: mockResourceGetterWithCluster,
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForVPCFromAppEnv).
					Return(nil, errors.New("error getting subnets"))
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			wantedError: fmt.Errorf(fmtErrPublicSubnetsFromEnv, inEnv, errors.New("error getting subnets")),
		},
		"no subnet is found": {
			mockResourceGetter: mockResourceGetterWithCluster,
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForVPCFromAppEnv).
					Return([]string{}, nil)
				m.EXPECT().SecurityGroups(gomock.Any()).AnyTimes()
			},
			mockStarter: mockStarterNotRun,
			wantedError: errNoSubnetFound,
		},
		"failed to get security groups": {
			mockResourceGetter: mockResourceGetterWithCluster,
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(gomock.Any()).Return([]string{"subnet-1"}, nil)
				m.EXPECT().SecurityGroups(filtersForVPCFromAppEnv).
					Return(nil, errors.New("error getting security groups"))
			},
			mockStarter: mockStarterNotRun,
			wantedError: fmt.Errorf(fmtErrSecurityGroupsFromEnv, inEnv, errors.New("error getting security groups")),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",

			mockResourceGetter: mockResourceGetterWithCluster,
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForVPCFromAppEnv).Return([]string{"subnet-1", "subnet-2"}, nil)
				m.EXPECT().SecurityGroups(filtersForVPCFromAppEnv).Return([]string{"sg-1", "sg-2"}, nil)
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
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

			mockResourceGetter: mockResourceGetterWithCluster,
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().PublicSubnetIDs(filtersForVPCFromAppEnv).Return([]string{"subnet-1", "subnet-2"}, nil)
				m.EXPECT().SecurityGroups(filtersForVPCFromAppEnv).Return([]string{"sg-1", "sg-2"}, nil)
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

			mockVPCGetter := mocks.NewMockVPCGetter(ctrl)
			mockResourceGetter := mocks.NewMockResourceGetter(ctrl)
			mockStarter := mocks.NewMockTaskRunner(ctrl)

			tc.mockVPCGetter(mockVPCGetter)
			tc.mockResourceGetter(mockResourceGetter)
			tc.mockStarter(mockStarter)

			task := &EnvRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				App: inApp,
				Env: inEnv,

				VPCGetter:     mockVPCGetter,
				ClusterGetter: mockResourceGetter,
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
