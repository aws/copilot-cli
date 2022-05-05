// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_Validate(t *testing.T) {
	mockVPCCIDR := IPNet("10.0.0.0/16")
	testCases := map[string]struct {
		in                   Environment
		wantedErrorMsgPrefix string
	}{
		"malformed network": {
			in: Environment{
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							ID:   stringP("vpc-123"),
							CIDR: &mockVPCCIDR,
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"succeed on empty config": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentNetworkConfig_Validate(t *testing.T) {
	mockVPCCIDR := IPNet("10.0.0.0/16")
	testCases := map[string]struct {
		in                   environmentNetworkConfig
		wantedErrorMsgPrefix string
	}{
		"malformed vpc": {
			in: environmentNetworkConfig{
				VPC: environmentVPCConfig{
					ID:   stringP("vpc-123"),
					CIDR: &mockVPCCIDR,
				},
			},
			wantedErrorMsgPrefix: `validate "vpc": `,
		},
		"succeed on empty config": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentVPCConfig_Validate(t *testing.T) {
	var (
		mockVPCCIDR            = IPNet("10.0.0.0/16")
		mockPublicSubnet1CIDR  = IPNet("10.0.0.0/24")
		mockPublicSubnet2CIDR  = IPNet("10.0.1.0/24")
		mockPublicSubnet3CIDR  = IPNet("10.0.2.0/24")
		mockPrivateSubnet1CIDR = IPNet("10.0.3.0/24")
		mockPrivateSubnet2CIDR = IPNet("10.0.4.0/24")
		mockPrivateSubnet3CIDR = IPNet("10.0.5.0/24")
	)
	testCases := map[string]struct {
		in                   environmentVPCConfig
		wantedErrorMsgPrefix string
		wantedErr            error
	}{
		"malformed subnets": {
			in: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-public-subnet-1"),
							CIDR:     &mockPublicSubnet1CIDR,
						},
						{
							SubnetID: aws.String("mock-public-subnet-2"),
							CIDR:     &mockPublicSubnet1CIDR,
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "subnets": `,
		},
		"error if vpc is both imported and configured": {
			in: environmentVPCConfig{
				ID:   aws.String("vpc-1234"),
				CIDR: &mockVPCCIDR,
			},
			wantedErr: errors.New(`cannot import VPC resources (with "id" fields) and customize VPC resources (with "cidr" and "az" fields) at the same time`),
		},
		"error if importing vpc while subnets are configured": {
			in: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": public[0] must include "id" field if the vpc is imported`),
		},
		"error if importing vpc while no subnet is imported": {
			in: environmentVPCConfig{
				ID:      aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{},
			},
			wantedErr: errors.New(`validate "subnets": VPC must have subnets in order to proceed with environment creation`),
		},
		"error if importing vpc while only one private subnet is imported": {
			in: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-public-subnet-1"),
						},
					},
					Private: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-private-subnet-1"),
						},
						{
							SubnetID: aws.String("mock-private-subnet-2"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": validate "public": at least two public subnets must be imported to enable Load Balancing`),
		},
		"error if importing vpc while only one public subnet is imported": {
			in: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-public-subnet-1"),
						},
						{
							SubnetID: aws.String("mock-public-subnet-2"),
						},
					},
					Private: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-private-subnet-1"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": validate "private": at least two private subnets must be imported`),
		},
		"error if configuring vpc without enough azs": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2a"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2a"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": require at least 2 availability zones`),
		},
		"error if configuring vpc while some subnets are imported": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							SubnetID: aws.String("mock-public-subnet-1"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": public[1] must include "cidr" and "az" fields if the vpc is configured`),
		},
		"error if configuring vpc while azs do not match between private and public subnets": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2c"),
						},
					},
				},
			},
			wantedErr: errors.New("validate \"subnets\": public subnets and private subnets do not span the same availability zones"),
		},
		"error if configuring vpc while the number of public subnet CIDR does not match the number of azs": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
						{
							CIDR: &mockPublicSubnet3CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": validate "public": number of public subnet CIDRs (3) does not match number of AZs (2)`),
		},
		"error if configuring vpc while the number of private subnet CIDR does not match the number of azs": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
						{
							CIDR: &mockPrivateSubnet3CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
				},
			},
			wantedErr: errors.New(`validate "subnets": validate "private": number of private subnet CIDRs (3) does not match number of AZs (2)`),
		},
		"succeed on imported vpc": {
			in: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-public-subnet-1"),
						},
						{
							SubnetID: aws.String("mock-public-subnet-2"),
						},
					},
					Private: []subnetConfiguration{
						{
							SubnetID: aws.String("mock-private-subnet-1"),
						},
						{
							SubnetID: aws.String("mock-private-subnet-2"),
						},
					},
				},
			},
		},
		"succeed on managed vpc": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
						{
							CIDR: &mockPublicSubnet3CIDR,
							AZ:   aws.String("us-east-2c"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
						{
							CIDR: &mockPrivateSubnet3CIDR,
							AZ:   aws.String("us-east-2c"),
						},
					},
				},
			},
		},
		"succeed on empty config": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedErr == nil && tc.wantedErrorMsgPrefix == "" {
				require.NoError(t, gotErr)
			}
			if tc.wantedErr != nil {
				require.EqualError(t, tc.wantedErr, gotErr.Error())
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			}
		})
	}
}

func TestSubnetsConfiguration_Validate(t *testing.T) {
	var (
		mockPublicSubnet1CIDR  = IPNet("10.0.0.0/24")
		mockPrivateSubnet1CIDR = IPNet("10.0.3.0/24")
	)
	testCases := map[string]struct {
		in                   subnetsConfiguration
		wantedErrorMsgPrefix string
	}{
		"malformed public subnets": {
			in: subnetsConfiguration{
				Public: []subnetConfiguration{
					{
						CIDR:     &mockPublicSubnet1CIDR,
						SubnetID: aws.String("mock-public-subnet-1"),
					},
				},
			},
			wantedErrorMsgPrefix: `validate "public[0]": `,
		},
		"malformed private subnets": {
			in: subnetsConfiguration{
				Private: []subnetConfiguration{
					{
						CIDR:     &mockPrivateSubnet1CIDR,
						SubnetID: aws.String("mock-private-subnet-1"),
					},
				},
			},
			wantedErrorMsgPrefix: `validate "private[0]": `,
		},
		"success": {
			in: subnetsConfiguration{
				Public: []subnetConfiguration{
					{
						SubnetID: aws.String("mock-public-subnet-1"),
					},
				},
				Private: []subnetConfiguration{
					{
						SubnetID: aws.String("mock-private-subnet-1"),
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestSubnetConfiguration_Validate(t *testing.T) {
	mockCIDR := IPNet("10.0.0.0/24")
	testCases := map[string]struct {
		in          subnetConfiguration
		wantedError error
	}{
		"error if id and cidr are both specified": {
			in: subnetConfiguration{
				SubnetID: aws.String("mock-subnet-1"),
				CIDR:     &mockCIDR,
			},
			wantedError: &errFieldMutualExclusive{
				firstField:  "id",
				secondField: "cidr",
				mustExist:   false,
			},
		},
		"error if id and az are both specified": {
			in: subnetConfiguration{
				SubnetID: aws.String("mock-subnet-1"),
				AZ:       aws.String("us-east-2a"),
			},
			wantedError: &errFieldMutualExclusive{
				firstField:  "id",
				secondField: "az",
				mustExist:   false,
			},
		},
		"succeed with id": {
			in: subnetConfiguration{
				SubnetID: aws.String("mock-subnet-1"),
			},
		},
		"succeed with cidr": {
			in: subnetConfiguration{
				CIDR: &mockCIDR,
			},
		},
		"succeed with az": {
			in: subnetConfiguration{
				AZ: aws.String("us-east-2a"),
			},
		},
		"succeed with both cidr and az": {
			in: subnetConfiguration{
				AZ:   aws.String("us-east-2a"),
				CIDR: &mockCIDR,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedError != nil {
				require.Error(t, gotErr)
				require.EqualError(t, tc.wantedError, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentHTTPConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		in                   environmentHTTPConfig
		wantedErrorMsgPrefix string
	}{
		"malformed certificates": {
			in: environmentHTTPConfig{
				Public: publicHTTPConfig{
					Certificates: []string{"arn:aws:weird-little-arn"},
				},
			},
			wantedErrorMsgPrefix: `validate "certificates[0]": `,
		},
		"success": {
			in: environmentHTTPConfig{
				Public: publicHTTPConfig{
					Certificates: []string{"arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.Validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
