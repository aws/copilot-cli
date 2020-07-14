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

func TestRunner_RunInDefaultVPC(t *testing.T) {
	testCases := map[string]struct {
		count     int
		groupName string

		mockVPCGetter     func(m *mocks.MockvpcGetter)
		mockClusterGetter func(m *mocks.MockdefaultClusterGetter)
		mockStarter       func(m *mocks.MocktaskRunner)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get default cluster": {
			mockClusterGetter: func(m *mocks.MockdefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("", errors.New("error getting cluster"))
			},
			mockVPCGetter: func(m *mocks.MockvpcGetter) {
				m.EXPECT().GetSubnetIDs().AnyTimes()
				m.EXPECT().GetSecurityGroups().AnyTimes()
			},
			mockStarter: func(m *mocks.MocktaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf("get default cluster: error getting cluster"),
		},
		"failed to get subnet": {
			mockClusterGetter: func(m *mocks.MockdefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockVPCGetter: func(m *mocks.MockvpcGetter) {
				m.EXPECT().GetSubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Return(nil, errors.New("error getting subnets"))
			},
			mockStarter: func(m *mocks.MocktaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Times(0)
			},
			wantedError: errors.New("get default subnet IDs: error getting subnets"),
		},
		"failed to kick off task": {
			count:     1,
			groupName: "my-task",
			mockClusterGetter: func(m *mocks.MockdefaultClusterGetter) {
				m.EXPECT().DefaultCluster().Return("cluster-1", nil)
			},
			mockVPCGetter: func(m *mocks.MockvpcGetter) {
				m.EXPECT().GetSubnetIDs([]ec2.Filter{ec2.FilterForDefaultVPCSubnets}).Return([]string{"subnet-1"}, nil)
			},
			mockStarter: func(m *mocks.MocktaskRunner) {
				m.EXPECT().RunTask(gomock.Any()).Return(nil, errors.New("error running task"))
			},
			wantedError: errors.New("run task my-task: error running task"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVpcGetter := mocks.NewMockvpcGetter(ctrl)
			mockClusterGetter := mocks.NewMockdefaultClusterGetter(ctrl)
			mockStarter := mocks.NewMocktaskRunner(ctrl)

			tc.mockVPCGetter(mockVpcGetter)
			tc.mockClusterGetter(mockClusterGetter)
			tc.mockStarter(mockStarter)

			runner := &Runner{
				count:     tc.count,
				groupName: tc.groupName,

				vpcGetter:     mockVpcGetter,
				clusterGetter: mockClusterGetter,
				starter:       mockStarter,
			}

			arns, err := runner.RunInDefaultVPC()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}
