// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
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

func TestEnvironmentConfig_validate(t *testing.T) {
	mockPublicSubnet1CIDR := IPNet("10.0.0.0/24")
	mockPublicSubnet2CIDR := IPNet("10.0.1.0/24")
	mockPrivateSubnet1CIDR := IPNet("10.0.3.0/24")
	mockPrivateSubnet2CIDR := IPNet("10.0.4.0/24")
	testCases := map[string]struct {
		in          EnvironmentConfig
		wantedError string
	}{
		"error if internal ALB subnet placement specified with adjusted vpc": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						CIDR: ipNetP("apple cider"),
						Subnets: subnetsConfiguration{
							Public: []subnetConfiguration{
								{
									CIDR: &mockPublicSubnet1CIDR,
									AZ:   aws.String("us-east-2a"),
								},
								{
									CIDR: &mockPublicSubnet2CIDR,
									AZ:   aws.String("us-east-1b"),
								},
							},
							Private: []subnetConfiguration{
								{
									CIDR: &mockPrivateSubnet1CIDR,
									AZ:   aws.String("us-east-2a"),
								},
								{
									CIDR: &mockPrivateSubnet2CIDR,
									AZ:   aws.String("us-east-1b"),
								},
							},
						},
					},
				},

				HTTPConfig: EnvironmentHTTPConfig{
					Private: privateHTTPConfig{
						InternalALBSubnets: []string{"mockSubnet"},
					},
				},
			},
			wantedError: "in order to specify internal ALB subnet placement, subnets must be imported",
		},
		"error if invalid security group config": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						SecurityGroupConfig: securityGroupConfig{
							Ingress: []securityGroupRule{
								{
									IpProtocol: "tcp",
									Ports: portsConfig{
										Port: aws.Int(80),
									},
								},
							},
						},
					},
				},
			},
			wantedError: "validate \"security_group\": validate ingress[0]: \"cidr\" must be specified",
		},
		"valid security group config": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						SecurityGroupConfig: securityGroupConfig{
							Ingress: []securityGroupRule{
								{
									CidrIP:     "0.0.0.0",
									IpProtocol: "tcp",
									Ports: portsConfig{
										Range: (*IntRangeBand)(aws.String("1-10")),
									},
								},
							},
						},
					},
				},
			},
		},
		"invalid ports value in security group config": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						SecurityGroupConfig: securityGroupConfig{
							Ingress: []securityGroupRule{
								{
									CidrIP:     "0.0.0.0",
									IpProtocol: "tcp",
									Ports: portsConfig{
										Range: (*IntRangeBand)(aws.String("1-10-10")),
									},
								},
							},
						},
					},
				},
			},
			wantedError: "validate \"security_group\": validate ingress[0]: invalid range value 1-10-10: valid format is ${from_port}-${to_port}",
		},
		"valid security group config without ports": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						SecurityGroupConfig: securityGroupConfig{
							Ingress: []securityGroupRule{
								{
									CidrIP:     "0.0.0.0",
									IpProtocol: "tcp",
								},
							},
						},
					},
				},
			},
			wantedError: "validate \"security_group\": validate ingress[0]: \"ports\" must be specified",
		},
		"error if security group ingress is limited to a cdn distribution not enabled": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Enabled: aws.Bool(false),
				},
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
							DeprecatedIngress: DeprecatedIngress{
								RestrictiveIngress: RestrictiveIngress{
									CDNIngress: aws.Bool(true),
								},
							},
						},
					},
				},
			},
			wantedError: "CDN must be enabled to limit security group ingress to CloudFront",
		},
		"valid vpc flowlogs with default retention": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						FlowLogs: Union[*bool, VPCFlowLogsArgs]{
							Basic: aws.Bool(true),
						},
					},
				},
			},
		},
		"valid vpc flowlogs with a specified retention": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						FlowLogs: Union[*bool, VPCFlowLogsArgs]{
							Advanced: VPCFlowLogsArgs{
								Retention: aws.Int(30),
							},
						},
					},
				},
			},
		},
		"valid elb access logs config with bucket_prefix": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						ELBAccessLogs: ELBAccessLogsArgsOrBool{
							AdvancedConfig: ELBAccessLogsArgs{
								Prefix: aws.String("prefix"),
							},
						},
					},
				},
			},
		},
		"valid elb access logs config with both bucket_prefix and bucket_name": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						ELBAccessLogs: ELBAccessLogsArgsOrBool{
							AdvancedConfig: ELBAccessLogsArgs{
								Prefix:     aws.String("prefix"),
								BucketName: aws.String("bucketName"),
							},
						},
					},
				},
			},
		},
		"error if cdn cert specified, cdn not terminating tls, and public certs not specified": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Config: AdvancedCDNConfig{
						Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
					},
				},
			},
			wantedError: `"cdn.terminate_tls" must be true if "cdn.certificate" is set without "http.public.certificates"`,
		},
		"success if cdn cert specified, cdn terminating tls, and no public certs": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Config: AdvancedCDNConfig{
						Certificate:  aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
						TerminateTLS: aws.Bool(true),
					},
				},
			},
		},
		"error if cdn cert not specified but public certs imported": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Enabled: aws.Bool(true),
				},
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						Certificates: []string{"arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"},
					},
				},
			},
			wantedError: "\"cdn.certificate\" must be specified if \"http.public.certificates\" and \"cdn\" are specified",
		},
		"error if subnets specified for internal ALB placement don't exist": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						ID: aws.String("mockID"),
						Subnets: subnetsConfiguration{
							Private: []subnetConfiguration{
								{SubnetID: aws.String("existentSubnet")},
								{SubnetID: aws.String("anotherExistentSubnet")},
							},
						},
					},
				},
				HTTPConfig: EnvironmentHTTPConfig{
					Private: privateHTTPConfig{
						InternalALBSubnets: []string{"nonexistentSubnet"},
					},
				},
			},
			wantedError: "subnet(s) specified for internal ALB placement not imported",
		},
		"valid case with internal ALB placement": {
			in: EnvironmentConfig{
				Network: environmentNetworkConfig{
					VPC: environmentVPCConfig{
						ID: aws.String("mockID"),
						Subnets: subnetsConfiguration{
							Private: []subnetConfiguration{
								{SubnetID: aws.String("existentSubnet")},
								{SubnetID: aws.String("anotherExistentSubnet")},
							},
							Public: []subnetConfiguration{
								{SubnetID: aws.String("publicSubnet1")},
								{SubnetID: aws.String("publicSubnet2")},
							},
						},
					},
				},
				HTTPConfig: EnvironmentHTTPConfig{
					Private: privateHTTPConfig{
						InternalALBSubnets: []string{"existentSubnet", "anotherExistentSubnet"},
					},
				},
			},
		},
		"returns error when http private config with deprecated and a new ingress field": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Private: privateHTTPConfig{
						Ingress: RelaxedIngress{
							VPCIngress: aws.Bool(true),
						},
						DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
							DeprecatedIngress: DeprecatedIngress{
								VPCIngress: aws.Bool(true),
							},
						},
					},
				},
			},
			wantedError: "validate \"http config\": validate \"private\": must specify one, not both, of \"private.http.security_groups.ingress\" and \"private.http.ingress\"",
		},
		"no error when http private config with a new ingress field": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Private: privateHTTPConfig{
						Ingress: RelaxedIngress{
							VPCIngress: aws.Bool(true),
						},
					},
				},
			},
		},
		"returns error when http public config with deprecated and a new ingress field": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						Ingress: RestrictiveIngress{
							CDNIngress: aws.Bool(true),
						},
						DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
							DeprecatedIngress: DeprecatedIngress{
								RestrictiveIngress: RestrictiveIngress{
									CDNIngress: aws.Bool(true),
								},
							},
						},
					},
				},
			},
			wantedError: "validate \"http config\": validate \"public\": must specify one, not both, of \"public.http.security_groups.ingress\" and \"public.http.ingress\"",
		},
		"no error when http public config with a new ingress field": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Enabled: aws.Bool(true),
				},
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						Ingress: RestrictiveIngress{
							CDNIngress: aws.Bool(true),
						},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()
			if tc.wantedError != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedError)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentNetworkConfig_validate(t *testing.T) {
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
			gotErr := tc.in.validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentVPCConfig_validate(t *testing.T) {
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
			wantedErr: errors.New(`validate "subnets" for an imported VPC: validate public[0]: "id" must be specified`),
		},
		"error if importing vpc while no subnet is imported": {
			in: environmentVPCConfig{
				ID:      aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{},
			},
			wantedErr: errors.New(`validate "subnets" for an imported VPC: VPC must have subnets in order to proceed with environment creation`),
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
			wantedErr: errors.New(`validate "subnets" for an imported VPC: validate "public": at least two public subnets must be imported to enable Load Balancing`),
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
			wantedErr: errors.New(`validate "subnets" for an imported VPC: validate "private": at least two private subnets must be imported`),
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
			wantedErr: errors.New(`validate "subnets" for an adjusted VPC: require at least 2 availability zones`),
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
			wantedErr: errors.New(`validate "subnets" for an adjusted VPC: validate public[1]: "cidr" must be specified`),
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
			wantedErr: errors.New("validate \"subnets\" for an adjusted VPC: public subnets and private subnets do not span the same availability zones"),
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
			wantedErr: errors.New(`validate "subnets" for an adjusted VPC: validate "public": number of public subnet CIDRs (3) does not match number of AZs (2)`),
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
			wantedErr: errors.New(`validate "subnets" for an adjusted VPC: validate "private": number of private subnet CIDRs (3) does not match number of AZs (2)`),
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
		"succeed on managed vpc that is fully adjusted ": {
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
		"succeed on managed vpc that does not adjust az": {
			in: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
						},
						{
							CIDR: &mockPublicSubnet3CIDR,
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet1CIDR,
						},
						{
							CIDR: &mockPrivateSubnet2CIDR,
						},
						{
							CIDR: &mockPrivateSubnet3CIDR,
						},
					},
				},
			},
		},
		"succeed on empty config": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()
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

func TestSubnetsConfiguration_validate(t *testing.T) {
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
			gotErr := tc.in.validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestCDNConfiguration_validate(t *testing.T) {
	testCases := map[string]struct {
		in                   EnvironmentCDNConfig
		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"valid if empty": {
			in: EnvironmentCDNConfig{},
		},
		"valid if bool specified": {
			in: EnvironmentCDNConfig{
				Enabled: aws.Bool(false),
			},
		},
		"success with cert without tls termination": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
				},
			},
		},
		"error if certificate invalid": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:weird-little-arn"),
				},
			},
			wantedErrorMsgPrefix: "parse cdn certificate:",
		},
		"error if certificate in invalid region": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-west-2:1111111:certificate/look-like-a-good-arn"),
				},
			},
			wantedError: errors.New("cdn certificate must be in region us-east-1"),
		},
		"error if static config invalid": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Static: CDNStaticConfig{
						Path: "something",
					},
				},
			},
			wantedErrorMsgPrefix: `validate "static_assets"`,
		},
		"success with cert and terminate tls": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Certificate:  aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
					TerminateTLS: aws.Bool(true),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else if tc.wantedError != nil {
				require.Error(t, gotErr)
				require.EqualError(t, tc.wantedError, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestCDNStaticConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		in          CDNStaticConfig
		wantedError error
	}{
		"valid if empty": {
			in: CDNStaticConfig{},
		},
		"invalid if alias is not specified": {
			in: CDNStaticConfig{
				Path: "something",
			},
			wantedError: fmt.Errorf(`"alias" must be specified`),
		},
		"invalid if location is not specified": {
			in: CDNStaticConfig{
				Alias: "example.com",
			},
			wantedError: fmt.Errorf(`"location" must be specified`),
		},
		"invalid if path is not specified": {
			in: CDNStaticConfig{
				Alias:    "example.com",
				Location: "s3url",
			},
			wantedError: fmt.Errorf(`"path" must be specified`),
		},
		"success": {
			in: CDNStaticConfig{
				Alias:    "example.com",
				Location: "static",
				Path:     "something",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()
			if tc.wantedError != nil {
				require.Error(t, gotErr)
				require.EqualError(t, tc.wantedError, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestSubnetConfiguration_validate(t *testing.T) {
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
			gotErr := tc.in.validate()
			if tc.wantedError != nil {
				require.Error(t, gotErr)
				require.EqualError(t, tc.wantedError, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvironmentHTTPConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		in                   EnvironmentHTTPConfig
		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"malformed public certificate": {
			in: EnvironmentHTTPConfig{
				Public: PublicHTTPConfig{
					Certificates: []string{"arn:aws:weird-little-arn"},
				},
			},
			wantedErrorMsgPrefix: `parse "certificates[0]": `,
		},
		"malformed private certificate": {
			in: EnvironmentHTTPConfig{
				Private: privateHTTPConfig{
					Certificates: []string{"arn:aws:weird-little-arn"},
				},
			},
			wantedErrorMsgPrefix: `parse "certificates[0]": `,
		},
		"success with public cert": {
			in: EnvironmentHTTPConfig{
				Public: PublicHTTPConfig{
					Certificates: []string{"arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"},
				},
			},
		},
		"success with private cert": {
			in: EnvironmentHTTPConfig{
				Private: privateHTTPConfig{
					Certificates: []string{"arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"},
				},
			},
		},
		"public http config with invalid security group ingress": {
			in: EnvironmentHTTPConfig{
				Public: PublicHTTPConfig{
					DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
						DeprecatedIngress: DeprecatedIngress{
							VPCIngress: aws.Bool(true),
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "public": a public load balancer already allows vpc ingress`),
		},
		"private http config with invalid security group ingress": {
			in: EnvironmentHTTPConfig{
				Private: privateHTTPConfig{
					DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
						DeprecatedIngress: DeprecatedIngress{
							RestrictiveIngress: RestrictiveIngress{
								CDNIngress: aws.Bool(true),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "private": an internal load balancer cannot have restrictive ingress fields`),
		},
		"public http config with invalid source ips": {
			in: EnvironmentHTTPConfig{
				Public: PublicHTTPConfig{
					DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
						DeprecatedIngress: DeprecatedIngress{
							RestrictiveIngress: RestrictiveIngress{SourceIPs: []IPNet{"1.1.1.invalidip"}},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "public": parse IPNet 1.1.1.invalidip: invalid CIDR address: 1.1.1.invalidip`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			} else if tc.wantedError != nil {
				require.Error(t, gotErr)
				require.EqualError(t, tc.wantedError, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
