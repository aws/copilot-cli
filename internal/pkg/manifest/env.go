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
	EnvironmentConfig `yaml:",inline"`

	parser template.Parser
}

// EnvironmentConfig holds the configuration for an environment.
type EnvironmentConfig struct {
	Network       environmentNetworkConfig `yaml:"network"`
	Observability environmentObservability `yaml:"observability"`
	HTTPConfig    environmentHTTPConfig    `yaml:"http"`
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

// ImportedVPC returns configurations that import VPC resources if there is any.
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

// ManagedVPC returns configurations that configure VPC resources if there is any.
func (v environmentVPCConfig) ManagedVPC() *template.ManagedVPC {
	// NOTE: In a managed VPC, #pub = #priv = #az.
	// Either the VPC isn't configured, or everything need to be explicitly configured.
	if !v.managedVPCCustomized() {
		return nil
	}
	publicSubnetCIDRs := make([]string, len(v.Subnets.Public))
	privateSubnetCIDRs := make([]string, len(v.Subnets.Public))
	azs := make([]string, len(v.Subnets.Public))

	// NOTE: sort based on `az`s to preserve the mappings between azs and public subnets, private subnets.
	// For example, if we have two subnets defined: public-subnet-1 ~ us-east-1a, and private-subnet-1 ~ us-east-1a.
	// We want to make sure that public-subnet-1, us-east-1a and private-subnet-1 are all at index 0 of in perspective lists.
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

type environmentObservability struct {
	ContainerInsights *bool `yaml:"container_insights"`
}

// IsEmpty returns true if there is no configuration to the environment's observability.
func (o environmentObservability) IsEmpty() bool {
	return o.ContainerInsights == nil
}

type environmentHTTPConfig struct {
	Internet internetHTTPConfig `yaml:"public"`
}

type internetHTTPConfig struct {
	Certificates []string `yaml:"certificates"`
}
