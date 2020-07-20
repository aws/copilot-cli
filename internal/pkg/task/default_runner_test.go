// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDefaultVPCRunner_Run(t *testing.T) {
	testCases := map[string]struct {
		count     int
		groupName string

		mockVPCGetter     func(m *mocks.MockVPCGetter)
		mockClusterGetter func(m *mocks.MockDefaultClusterGetter)
		mockStarter       func(m *mocks.MockTaskRunner)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get default cluster": {
			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("", errors.New("error getting cluster"))
			},
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs().AnyTimes()
				m.EXPECT().SecurityGroups().AnyTimes()
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: &errGetDefaultCluster{
				parentErr: errors.New("error getting cluster"),
			},
		},
		"failed to get subnet": {
			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Return(nil, errors.New("error getting subnets"))
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf(fmtErrDefaultSubnets, errors.New("error getting subnets")),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",
			mockClusterGetter: func(m *mocks.MockDefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockVPCGetter: func(m *mocks.MockVPCGetter) {
				m.EXPECT().SubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Return([]string{"subnet-1"}, nil)
			},
			mockStarter: func(m *mocks.MockTaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Return(nil, errors.New("error running task"))
			},
			wantedError: &errRunTask{
				groupName: "my-task",
				parentErr: errors.New("error running task"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVpcGetter := mocks.NewMockVPCGetter(ctrl)
			mockClusterGetter := mocks.NewMockDefaultClusterGetter(ctrl)
			mockStarter := mocks.NewMockTaskRunner(ctrl)

			tc.mockVPCGetter(mockVpcGetter)
			tc.mockClusterGetter(mockClusterGetter)
			tc.mockStarter(mockStarter)

			runner := &DefaultVPCRunner{
				Count:     tc.count,
				GroupName: tc.groupName,

				VPCGetter:     mockVpcGetter,
				ClusterGetter: mockClusterGetter,
				Starter:       mockStarter,
			}

			arns, err := runner.Run()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}
