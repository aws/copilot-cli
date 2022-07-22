// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/manifest/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type dynamicManifestMock struct {
	mockSubnetGetter *mocks.MocksubnetIDsGetter
}

func newMockMftWithTags() workloadManifest {
	mockMftWithTags := newDefaultBackendService()
	mockMftWithTags.Network.VPC.Placement.Subnets.FromTags = Tags{"foo": StringSliceOrString{
		String: aws.String("bar"),
	}}
	return mockMftWithTags
}

func TestDynamicWorkloadManifest_Load(t *testing.T) {
	mockMft := newDefaultBackendService()
	testCases := map[string]struct {
		inMft workloadManifest

		setupMocks func(m dynamicManifestMock)

		wantedSubnetIDs []string
		wantedError     error
	}{
		"error if fail to get subnet IDs from tags": {
			inMft: newMockMftWithTags(),

			setupMocks: func(m dynamicManifestMock) {
				m.mockSubnetGetter.EXPECT().SubnetIDs(gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("get subnet IDs: some error"),
		},
		"success with subnet IDs from tags": {
			inMft: newMockMftWithTags(),

			setupMocks: func(m dynamicManifestMock) {
				m.mockSubnetGetter.EXPECT().SubnetIDs(ec2.FilterForTags("foo", "bar")).Return([]string{"id1", "id2"}, nil)
			},

			wantedSubnetIDs: []string{"id1", "id2"},
		},
		"success with no subnets": {
			inMft: mockMft,

			setupMocks: func(m dynamicManifestMock) {},

			wantedSubnetIDs: []string{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := dynamicManifestMock{
				mockSubnetGetter: mocks.NewMocksubnetIDsGetter(ctrl),
			}
			tc.setupMocks(m)

			dyn := &DynamicWorkloadManifest{
				mft: tc.inMft,
				newSubnetIDsGetter: func(s *session.Session) subnetIDsGetter {
					return m.mockSubnetGetter
				},
			}
			err := dyn.Load(nil)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedSubnetIDs, dyn.mft.subnets().IDs)
			}
		})
	}
}
