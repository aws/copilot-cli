// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func TestFromEnvConfig(t *testing.T) {
	testCases := map[string]struct {
		in     *config.Environment
		wanted *Environment
	}{
		"converts configured VPC settings with availability zones after v1.14": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					VPCConfig: &config.AdjustVPC{
						CIDR:               "10.0.0.0/16",
						AZs:                []string{"us-west-2a", "us-west-2b", "us-west-2c"},
						PublicSubnetCIDRs:  []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"},
						PrivateSubnetCIDRs: []string{"10.0.3.0/24", "10.0.4.0/24", "10.0.5.0/24"},
					},
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							CIDR: ipNetP("10.0.0.0/16"),
							Subnets: subnetsConfiguration{
								Public: []subnetConfiguration{
									{
										CIDR: ipNetP("10.0.0.0/24"),
										AZ:   stringP("us-west-2a"),
									},
									{
										CIDR: ipNetP("10.0.1.0/24"),
										AZ:   stringP("us-west-2b"),
									},
									{
										CIDR: ipNetP("10.0.2.0/24"),
										AZ:   stringP("us-west-2c"),
									},
								},
								Private: []subnetConfiguration{
									{
										CIDR: ipNetP("10.0.3.0/24"),
										AZ:   stringP("us-west-2a"),
									},
									{
										CIDR: ipNetP("10.0.4.0/24"),
										AZ:   stringP("us-west-2b"),
									},
									{
										CIDR: ipNetP("10.0.5.0/24"),
										AZ:   stringP("us-west-2c"),
									},
								},
							},
						},
					},
				},
			},
		},
		"converts configured VPC settings without any availability zones set": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					VPCConfig: &config.AdjustVPC{
						CIDR:               "10.0.0.0/16",
						PublicSubnetCIDRs:  []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"},
						PrivateSubnetCIDRs: []string{"10.0.3.0/24", "10.0.4.0/24", "10.0.5.0/24"},
					},
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							CIDR: ipNetP("10.0.0.0/16"),
							Subnets: subnetsConfiguration{
								Public: []subnetConfiguration{
									{
										CIDR: ipNetP("10.0.0.0/24"),
									},
									{
										CIDR: ipNetP("10.0.1.0/24"),
									},
									{
										CIDR: ipNetP("10.0.2.0/24"),
									},
								},
								Private: []subnetConfiguration{
									{
										CIDR: ipNetP("10.0.3.0/24"),
									},
									{
										CIDR: ipNetP("10.0.4.0/24"),
									},
									{
										CIDR: ipNetP("10.0.5.0/24"),
									},
								},
							},
						},
					},
				},
			},
		},
		"converts imported VPC settings": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportVPC: &config.ImportVPC{
						ID:               "vpc-3f139646",
						PublicSubnetIDs:  []string{"pub1", "pub2", "pub3"},
						PrivateSubnetIDs: []string{"priv1", "priv2", "priv3"},
					},
				},
			},
			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							ID: stringP("vpc-3f139646"),
							Subnets: subnetsConfiguration{
								Public: []subnetConfiguration{
									{
										SubnetID: stringP("pub1"),
									},
									{
										SubnetID: stringP("pub2"),
									},
									{
										SubnetID: stringP("pub3"),
									},
								},
								Private: []subnetConfiguration{
									{
										SubnetID: stringP("priv1"),
									},
									{
										SubnetID: stringP("priv2"),
									},
									{
										SubnetID: stringP("priv3"),
									},
								},
							},
						},
					},
				},
			},
		},
		"converts imported certificates for a public load balancer": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportCertARNs: []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
					ImportVPC: &config.ImportVPC{
						PublicSubnetIDs: []string{"subnet1"},
					},
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							Subnets: subnetsConfiguration{
								Public: []subnetConfiguration{
									{
										SubnetID: aws.String("subnet1"),
										CIDR:     nil,
										AZ:       nil,
									},
								},
								Private: []subnetConfiguration{},
							},
						},
					},
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
						},
					},
				},
			},
		},
		"converts imported certificates for a public load balancer without an imported vpc": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportCertARNs: []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
						},
					},
				},
			},
		},
		"converts imported certificates for a private load balancer with subnet placement specified": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportCertARNs: []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
					ImportVPC: &config.ImportVPC{
						PrivateSubnetIDs: []string{"subnet1", "subnet2"},
					},
					InternalALBSubnets: []string{"subnet2"},
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
							Subnets: subnetsConfiguration{
								Private: []subnetConfiguration{
									{
										SubnetID: aws.String("subnet1"),
									},
									{
										SubnetID: aws.String("subnet2"),
									},
								},
								Public: []subnetConfiguration{},
							},
						},
					},
					HTTPConfig: EnvironmentHTTPConfig{
						Private: privateHTTPConfig{
							InternalALBSubnets: []string{"subnet2"},
							Certificates:       []string{"arn:aws:acm:region:account:certificate/certificate_ID_1", "arn:aws:acm:region:account:certificate/certificate_ID_2"},
						},
					},
				},
			},
		},
		"converts container insights": {
			in: &config.Environment{
				App:  "phonetool",
				Name: "test",
				Telemetry: &config.Telemetry{
					EnableContainerInsights: false,
				},
			},

			wanted: &Environment{
				Workload: Workload{
					Name: stringP("test"),
					Type: stringP("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Observability: environmentObservability{
						ContainerInsights: aws.Bool(false),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, FromEnvConfig(tc.in, nil))
		})
	}
}

func Test_UnmarshalEnvironment(t *testing.T) {
	var (
		mockVPCCIDR            = IPNet("10.0.0.0/16")
		mockPublicSubnet1CIDR  = IPNet("10.0.0.0/24")
		mockPublicSubnet2CIDR  = IPNet("10.0.1.0/24")
		mockPrivateSubnet1CIDR = IPNet("10.0.3.0/24")
		mockPrivateSubnet2CIDR = IPNet("10.0.4.0/24")
	)
	testCases := map[string]struct {
		inContent       string
		wantedStruct    *Environment
		wantedErrPrefix string
	}{
		"unmarshal with managed VPC": {
			inContent: `name: test
type: Environment

network:
    vpc:
        cidr: '10.0.0.0/16'       
        subnets:
            public:
                - cidr: '10.0.0.0/24'  
                  az: 'us-east-2a'      
                - cidr: '10.0.1.0/24'   
                  az: 'us-east-2b'      
            private:
                - cidr: '10.0.3.0/24'   
                  az: 'us-east-2a'      
                - cidr: '10.0.4.0/24'   
                  az: 'us-east-2b'  
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("test"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Network: environmentNetworkConfig{
						VPC: environmentVPCConfig{
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
								},
							},
						},
					},
				},
			},
		},
		"unmarshal with enable access logs": {
			inContent: `name: prod
type: Environment

http:
  public:
    access_logs: true`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							ELBAccessLogs: ELBAccessLogsArgsOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				},
			},
		},
		"unmarshal with advanced access logs": {
			inContent: `name: prod
type: Environment

http:
  public:
    access_logs:
      bucket_name: testbucket
      prefix: prefix`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							ELBAccessLogs: ELBAccessLogsArgsOrBool{
								AdvancedConfig: ELBAccessLogsArgs{
									Prefix:     aws.String("prefix"),
									BucketName: aws.String("testbucket"),
								},
							},
						},
					},
				},
			},
		},
		"unmarshal with observability": {
			inContent: `name: prod
type: Environment

observability:
    container_insights: true
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					Observability: environmentObservability{
						ContainerInsights: aws.Bool(true),
					},
				},
			},
		},
		"unmarshal with content delivery network bool": {
			inContent: `name: prod
type: Environment

cdn: true
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					CDNConfig: EnvironmentCDNConfig{
						Enabled: aws.Bool(true),
					},
				},
			},
		},
		"unmarshal with http": {
			inContent: `name: prod
type: Environment

http:
    public:
        certificates:
            - cert-1
            - cert-2
    private:
        security_groups:
            ingress:
                from_vpc: false
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"cert-1", "cert-2"},
						},
						Private: privateHTTPConfig{
							DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
								DeprecatedIngress: DeprecatedIngress{
									VPCIngress: aws.Bool(false),
								},
							},
						},
					},
				},
			},
		},
		"unmarshal with new http fields": {
			inContent: `name: prod
type: Environment
http:
    public:
        certificates:
            - cert-1
            - cert-2
    private:
      ingress:
        vpc: true
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"cert-1", "cert-2"},
						},
						Private: privateHTTPConfig{
							Ingress: RelaxedIngress{VPCIngress: aws.Bool(true)},
						},
					},
				},
			},
		},
		"unmarshal with new and old private http fields": {
			inContent: `name: prod
type: Environment
http:
    public:
        certificates:
            - cert-1
            - cert-2
    private:
      security_groups:
        ingress:
          from_vpc: true
      ingress:
        vpc: true
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"cert-1", "cert-2"},
						},
						Private: privateHTTPConfig{
							Ingress: RelaxedIngress{VPCIngress: aws.Bool(true)},
							DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
								DeprecatedIngress: DeprecatedIngress{
									VPCIngress: aws.Bool(true),
								},
							},
						},
					},
				},
			},
		},
		"unmarshal with new and old public http fields": {
			inContent: `name: prod
type: Environment
http:
    public:
      certificates:
        - cert-1
        - cert-2
      security_groups:
        ingress:
          restrict_to:
            cdn: true
      ingress:
        cdn: true
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"cert-1", "cert-2"},
							DeprecatedSG: DeprecatedALBSecurityGroupsConfig{
								DeprecatedIngress: DeprecatedIngress{
									RestrictiveIngress: RestrictiveIngress{
										CDNIngress: aws.Bool(true),
									},
								},
							},
							Ingress: RestrictiveIngress{CDNIngress: aws.Bool(true)},
						},
					},
				},
			},
		},
		"unmarshal with source_ips field in http.public": {
			inContent: `name: prod
type: Environment
http:
    public:
      certificates:
        - cert-1
        - cert-2
      security_groups:
        ingress:
          restrict_to:
            cdn: true
      ingress:
        source_ips:
          - 1.1.1.1
          - 2.2.2.2
`,
			wantedStruct: &Environment{
				Workload: Workload{
					Name: aws.String("prod"),
					Type: aws.String("Environment"),
				},
				EnvironmentConfig: EnvironmentConfig{
					HTTPConfig: EnvironmentHTTPConfig{
						Public: PublicHTTPConfig{
							Certificates: []string{"cert-1", "cert-2"},
							DeprecatedSG: DeprecatedALBSecurityGroupsConfig{DeprecatedIngress: DeprecatedIngress{RestrictiveIngress: RestrictiveIngress{CDNIngress: aws.Bool(true)}}},
							Ingress:      RestrictiveIngress{SourceIPs: []IPNet{"1.1.1.1", "2.2.2.2"}},
						},
					},
				},
			},
		},
		"fail to unmarshal": {
			inContent:       `watermelon in easter hay`,
			wantedErrPrefix: "unmarshal environment manifest: ",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := UnmarshalEnvironment([]byte(tc.inContent))
			if tc.wantedErrPrefix != "" {
				require.ErrorContains(t, gotErr, tc.wantedErrPrefix)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedStruct, got)
			}
		})
	}
}

func TestEnvironmentVPCConfig_ImportedVPC(t *testing.T) {
	testCases := map[string]struct {
		inVPCConfig environmentVPCConfig
		wanted      *template.ImportVPC
	}{
		"vpc not imported": {},
		"only public subnets imported": {
			inVPCConfig: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("subnet-123"),
						},
						{
							SubnetID: aws.String("subnet-456"),
						},
					},
				},
			},
			wanted: &template.ImportVPC{
				ID:              "vpc-1234",
				PublicSubnetIDs: []string{"subnet-123", "subnet-456"},
			},
		},
		"only private subnets imported": {
			inVPCConfig: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Private: []subnetConfiguration{
						{
							SubnetID: aws.String("subnet-123"),
						},
						{
							SubnetID: aws.String("subnet-456"),
						},
					},
				},
			},
			wanted: &template.ImportVPC{
				ID:               "vpc-1234",
				PrivateSubnetIDs: []string{"subnet-123", "subnet-456"},
			},
		},
		"both subnets imported": {
			inVPCConfig: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							SubnetID: aws.String("subnet-123"),
						},
						{
							SubnetID: aws.String("subnet-456"),
						},
					},
					Private: []subnetConfiguration{
						{
							SubnetID: aws.String("subnet-789"),
						},
						{
							SubnetID: aws.String("subnet-012"),
						},
					},
				},
			},
			wanted: &template.ImportVPC{
				ID:               "vpc-1234",
				PublicSubnetIDs:  []string{"subnet-123", "subnet-456"},
				PrivateSubnetIDs: []string{"subnet-789", "subnet-012"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.inVPCConfig.ImportedVPC()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentVPCConfig_ManagedVPC(t *testing.T) {
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
		inVPCConfig environmentVPCConfig
		wanted      *template.ManagedVPC
	}{
		"default vpc without custom configuration": {},
		"with imported vpc": {
			inVPCConfig: environmentVPCConfig{
				ID: aws.String("vpc-1234"),
			},
		},
		"ensure custom configuration is sorted by AZ": {
			inVPCConfig: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPublicSubnet3CIDR,
							AZ:   aws.String("us-east-2c"),
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet2CIDR,
							AZ:   aws.String("us-east-2b"),
						},
						{
							CIDR: &mockPrivateSubnet1CIDR,
							AZ:   aws.String("us-east-2a"),
						},
						{
							CIDR: &mockPrivateSubnet3CIDR,
							AZ:   aws.String("us-east-2c"),
						},
					},
				},
			},
			wanted: &template.ManagedVPC{
				CIDR:               string(mockVPCCIDR),
				AZs:                []string{"us-east-2a", "us-east-2b", "us-east-2c"},
				PublicSubnetCIDRs:  []string{string(mockPublicSubnet1CIDR), string(mockPublicSubnet2CIDR), string(mockPublicSubnet3CIDR)},
				PrivateSubnetCIDRs: []string{string(mockPrivateSubnet1CIDR), string(mockPrivateSubnet2CIDR), string(mockPrivateSubnet3CIDR)},
			},
		},
		"managed vpc without explicitly configured azs": {
			inVPCConfig: environmentVPCConfig{
				CIDR: &mockVPCCIDR,
				Subnets: subnetsConfiguration{
					Public: []subnetConfiguration{
						{
							CIDR: &mockPublicSubnet1CIDR,
						},
						{
							CIDR: &mockPublicSubnet3CIDR,
						},
						{
							CIDR: &mockPublicSubnet2CIDR,
						},
					},
					Private: []subnetConfiguration{
						{
							CIDR: &mockPrivateSubnet2CIDR,
						},
						{
							CIDR: &mockPrivateSubnet1CIDR,
						},
						{
							CIDR: &mockPrivateSubnet3CIDR,
						},
					},
				},
			},
			wanted: &template.ManagedVPC{
				CIDR:               string(mockVPCCIDR),
				PublicSubnetCIDRs:  []string{string(mockPublicSubnet1CIDR), string(mockPublicSubnet3CIDR), string(mockPublicSubnet2CIDR)},
				PrivateSubnetCIDRs: []string{string(mockPrivateSubnet2CIDR), string(mockPrivateSubnet1CIDR), string(mockPrivateSubnet3CIDR)},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.inVPCConfig.ManagedVPC()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentVPCConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     environmentVPCConfig
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty when VPC ID is provided": {
			in: environmentVPCConfig{
				ID: aws.String("mock-vpc-id"),
			},
		},
		"not empty when flowlog is on": {
			in: environmentVPCConfig{
				FlowLogs: Union[*bool, VPCFlowLogsArgs]{
					Basic: aws.Bool(true),
				},
			},
			wanted: true,
		},
		"not empty when flowlog with specific retention": {
			in: environmentVPCConfig{
				FlowLogs: Union[*bool, VPCFlowLogsArgs]{
					Advanced: VPCFlowLogsArgs{
						Retention: aws.Int(60),
					},
				},
			},
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestSubnetsConfiguration_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     subnetsConfiguration
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty": {
			in: subnetsConfiguration{
				Public: []subnetConfiguration{
					{
						SubnetID: aws.String("mock-subnet-id"),
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestCDNStaticConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     CDNStaticConfig
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty": {
			in: CDNStaticConfig{
				Path: "something",
			},
			wanted: false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentHTTPConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     EnvironmentHTTPConfig
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty": {
			in: EnvironmentHTTPConfig{
				Public: PublicHTTPConfig{
					Certificates: []string{"mock-cert"},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestPublicHTTPConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     PublicHTTPConfig
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty when Certificates are attached": {
			in: PublicHTTPConfig{
				Certificates: []string{"mock-cert-1"},
			},
		},
		"not empty when SSL Policy is present": {
			in: PublicHTTPConfig{
				SSLPolicy: aws.String("mock-ELB-ELBSecurityPolicy"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestPrivateHTTPConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     privateHTTPConfig
		wanted bool
	}{
		"empty": {
			wanted: true,
		},
		"not empty when Certificates are attached": {
			in: privateHTTPConfig{
				InternalALBSubnets: []string{"mock-subnet-1"},
			},
		},
		"not empty when SSL Policy is present": {
			in: privateHTTPConfig{
				SSLPolicy: aws.String("mock-ELB-ELBSecurityPolicy"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentObservability_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     environmentObservability
		wanted bool
	}{
		"empty": {
			in:     environmentObservability{},
			wanted: true,
		},
		"not empty": {
			in: environmentObservability{
				ContainerInsights: aws.Bool(false),
			},
			wanted: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentCDNConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     EnvironmentCDNConfig
		wanted bool
	}{
		"empty": {
			in:     EnvironmentCDNConfig{},
			wanted: true,
		},
		"not empty": {
			in: EnvironmentCDNConfig{
				Enabled: aws.Bool(false),
			},
			wanted: false,
		},
		"advanced not empty": {
			in: EnvironmentCDNConfig{
				Config: AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
				},
			},
			wanted: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.IsEmpty()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentConfig_CDNEnabled(t *testing.T) {
	testCases := map[string]struct {
		in     EnvironmentConfig
		wanted bool
	}{
		"enabled via bool": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Enabled: aws.Bool(true),
				},
			},
			wanted: true,
		},
		"enabled via config": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Config: AdvancedCDNConfig{
						Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
					},
				},
			},
			wanted: true,
		},
		"not enabled because empty": {
			in:     EnvironmentConfig{},
			wanted: false,
		},
		"not enabled via bool": {
			in: EnvironmentConfig{
				CDNConfig: EnvironmentCDNConfig{
					Enabled: aws.Bool(false),
				},
			},
			wanted: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.in.CDNEnabled()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestEnvironmentConfig_ELBAccessLogs(t *testing.T) {
	testCases := map[string]struct {
		in            EnvironmentConfig
		wantedFlag    bool
		wantedConfigs *ELBAccessLogsArgs
	}{
		"enabled via bool": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						ELBAccessLogs: ELBAccessLogsArgsOrBool{
							Enabled: aws.Bool(true),
						},
					},
				},
			},
			wantedFlag:    true,
			wantedConfigs: nil,
		},
		"disabled via bool": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						ELBAccessLogs: ELBAccessLogsArgsOrBool{
							Enabled: aws.Bool(false),
						},
					},
				},
			},
			wantedFlag:    false,
			wantedConfigs: nil,
		},
		"advanced access logs config": {
			in: EnvironmentConfig{
				HTTPConfig: EnvironmentHTTPConfig{
					Public: PublicHTTPConfig{
						ELBAccessLogs: ELBAccessLogsArgsOrBool{
							AdvancedConfig: ELBAccessLogsArgs{
								Prefix:     aws.String("prefix"),
								BucketName: aws.String("bucketname"),
							},
						},
					},
				},
			},
			wantedFlag: true,
			wantedConfigs: &ELBAccessLogsArgs{
				BucketName: aws.String("bucketname"),
				Prefix:     aws.String("prefix"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			elbAccessLogs, flag := tc.in.ELBAccessLogs()
			require.Equal(t, tc.wantedFlag, flag)
			require.Equal(t, tc.wantedConfigs, elbAccessLogs)
		})
	}
}
