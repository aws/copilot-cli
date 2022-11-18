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

func TestDynamicWorkload_ApplyEnv(t *testing.T) {
	tests := map[string]struct {
		manifest string
		expected func() any
	}{
		"update rdws http.private true -> false": {
			manifest: `
type: Request-Driven Web Service

http:
  private: true

environments:
  test:
    http:
      private: false
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = BasicToUnion[*bool, VPCEndpoint](aws.Bool(false))
				return c
			},
		},
		"update rdws http.private false -> true": {
			manifest: `
type: Request-Driven Web Service

http:
  private: false

environments:
  test:
    http:
      private: true
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = BasicToUnion[*bool, VPCEndpoint](aws.Bool(true))
				return c
			},
		},
		"update rdws http.private false -> VPCEndpoint": {
			manifest: `
type: Request-Driven Web Service

http:
  private: false

environments:
  test:
    http:
      private:
        endpoint: vpce-1234
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = AdvancedToUnion[*bool](VPCEndpoint{
					Endpoint: aws.String("vpce-1234"),
				})
				return c
			},
		},
		"update rdws http.private VPCEndpoint -> false": {
			manifest: `
type: Request-Driven Web Service

http:
  private:
    endpoint: vpce-1234

environments:
  test:
    http:
      private: false
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = BasicToUnion[*bool, VPCEndpoint](aws.Bool(false))
				return c
			},
		},
		"update rdws alias, no http.private update": {
			manifest: `
type: Request-Driven Web Service

http:
  private: false

environments:
  test:
    http:
      alias: example.com
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = BasicToUnion[*bool, VPCEndpoint](aws.Bool(false))
				c.Alias = aws.String("example.com")
				return c
			},
		},
		"update rdws cpu, no http.private update": {
			manifest: `
type: Request-Driven Web Service

http:
  private: true

environments:
  test:
    cpu: 2048
`,
			expected: func() any {
				c := newDefaultRequestDrivenWebService().RequestDrivenWebServiceConfig
				c.Private = BasicToUnion[*bool, VPCEndpoint](aws.Bool(true))
				c.InstanceConfig.CPU = aws.Int(2048)
				return c
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mft, err := UnmarshalWorkload([]byte(tc.manifest))
			require.NoError(t, err)

			mft, err = mft.ApplyEnv("test")
			require.NoError(t, err)

			switch v := mft.Manifest().(type) {
			case *RequestDrivenWebService:
				require.Equal(t, tc.expected(), v.RequestDrivenWebServiceConfig)
			}
		})
	}
}
