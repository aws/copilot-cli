// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEnvironment_UnmarshalYaml(t *testing.T) {
	var (
		mockVPCCIDR            = IPNet("10.0.0.0/16")
		mockPublicSubnet1CIDR  = IPNet("10.0.0.0/24")
		mockPublicSubnet2CIDR  = IPNet("10.0.1.0/24")
		mockPrivateSubnet1CIDR = IPNet("10.0.3.0/24")
		mockPrivateSubnet2CIDR = IPNet("10.0.4.0/24")
	)
	testCases := map[string]struct {
		inContent    string
		wantedStruct Environment
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
			wantedStruct: Environment{
				Workload: Workload{
					Name: aws.String("test"),
					Type: aws.String("Environment"),
				},
				environmentConfig: environmentConfig{
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var env Environment
			err := yaml.Unmarshal([]byte(tc.inContent), &env)
			require.NoError(t, err)
			require.Equal(t, tc.wantedStruct, env)
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
		"with custom configuration to managed vpc": {
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.inVPCConfig.ManagedVPC()
			require.Equal(t, tc.wanted, got)
		})
	}
}
