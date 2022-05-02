// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Environment is the manifest configuration for an environment.
type Environment struct {
	Workload          `yaml:",inline"`
	environmentConfig `yaml:",inline"`

	parser template.Parser
}

type environmentConfig struct {
	Network environmentNetworkConfig `yaml:"network"`
}

type environmentNetworkConfig struct {
	VPC environmentVPCConfig `yaml:"vpc"`
}

type environmentVPCConfig struct {
	ID      *string              `yaml:"id"`
	CIDR    *IPNet               `yaml:"cidr"`
	Subnets subnetsConfiguration `yaml:"subnets"`
}

func (v environmentVPCConfig) imported() bool {
	return aws.StringValue(v.ID) != ""
}

func (v environmentVPCConfig) managedVPCCustomized() bool {
	return aws.StringValue((*string)(v.CIDR)) != ""
}

func (v environmentVPCConfig) ImportedVPC() *template.ImportVPC {
	if !v.imported() {
		return nil
	}
	var publicSubnetIDs, privateSubnetIDs []string
	for _, subnet := range v.Subnets.Public {
		publicSubnetIDs = append(publicSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	for _, subnet := range v.Subnets.Private {
		privateSubnetIDs = append(privateSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	return &template.ImportVPC{
		ID:               aws.StringValue(v.ID),
		PublicSubnetIDs:  publicSubnetIDs,
		PrivateSubnetIDs: privateSubnetIDs,
	}
}

func (v environmentVPCConfig) ManagedVPC() *template.ManagedVPC {
	// NOTE: In a managed VPC, #pub = #priv = #az.
	// Either the VPC isn't configured, or everything need to be explicitly configured.
	if !v.managedVPCCustomized() {
		return nil
	}
	publicSubnetCIDRs := make([]string, len(v.Subnets.Public))
	privateSubnetCIDRs := make([]string, len(v.Subnets.Public))
	azs := make([]string, len(v.Subnets.Public))

	sort.Slice(v.Subnets.Public, func(i, j int) bool {
		return aws.StringValue(v.Subnets.Public[i].AZ) < aws.StringValue(v.Subnets.Public[j].AZ)
	})
	sort.Slice(v.Subnets.Private, func(i, j int) bool {
		return aws.StringValue(v.Subnets.Private[i].AZ) < aws.StringValue(v.Subnets.Private[j].AZ)
	})
	for idx, subnet := range v.Subnets.Public {
		publicSubnetCIDRs[idx] = aws.StringValue((*string)(subnet.CIDR))
		privateSubnetCIDRs[idx] = aws.StringValue((*string)(v.Subnets.Private[idx].CIDR))
		azs[idx] = aws.StringValue(subnet.AZ)
	}
	return &template.ManagedVPC{
		CIDR:               aws.StringValue((*string)(v.CIDR)),
		AZs:                azs,
		PublicSubnetCIDRs:  publicSubnetCIDRs,
		PrivateSubnetCIDRs: privateSubnetCIDRs,
	}
}

type subnetsConfiguration struct {
	Public  []subnetConfiguration `yaml:"public"`
	Private []subnetConfiguration `yaml:"private"`
}

type subnetConfiguration struct {
	SubnetID *string `yaml:"id"`
	CIDR     *IPNet  `yaml:"cidr"`
	AZ       *string `yaml:"az"`
}
