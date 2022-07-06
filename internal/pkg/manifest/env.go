// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"sort"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

// EnvironmentManifestType identifies that the type of manifest is environment manifest.
const EnvironmentManifestType = "Environment"

// Environment is the manifest configuration for an environment.
type Environment struct {
	Workload          `yaml:",inline"`
	EnvironmentConfig `yaml:",inline"`

	parser template.Parser
}

// FromEnvConfig transforms an environment configuration into a manifest.
func FromEnvConfig(cfg *config.Environment, parser template.Parser) *Environment {
	var vpc environmentVPCConfig
	vpc.loadVPCConfig(cfg.CustomConfig)

	var http environmentHTTPConfig
	http.loadLBConfig(cfg.CustomConfig)

	var obs environmentObservability
	obs.loadObsConfig(cfg.Telemetry)

	return &Environment{
		Workload: Workload{
			Name: stringP(cfg.Name),
			Type: stringP(EnvironmentManifestType),
		},
		EnvironmentConfig: EnvironmentConfig{
			Network: environmentNetworkConfig{
				VPC: vpc,
			},
			HTTPConfig:    http,
			Observability: obs,
		},
		parser: parser,
	}
}

// EnvironmentConfig holds the configuration for an environment.
type EnvironmentConfig struct {
	Network       environmentNetworkConfig `yaml:"network,omitempty,flow"`
	Observability environmentObservability `yaml:"observability,omitempty,flow"`
	HTTPConfig    environmentHTTPConfig    `yaml:"http,omitempty,flow"`
}

type environmentNetworkConfig struct {
	VPC environmentVPCConfig `yaml:"vpc,omitempty"`
}

type environmentVPCConfig struct {
	ID      *string              `yaml:"id"`
	CIDR    *IPNet               `yaml:"cidr"`
	Subnets subnetsConfiguration `yaml:"subnets,omitempty"`
}

// IsEmpty returns true if vpc is not configured.
func (cfg environmentVPCConfig) IsEmpty() bool {
	return cfg.ID == nil && cfg.CIDR == nil && cfg.Subnets.IsEmpty()
}

func (cfg *environmentVPCConfig) loadVPCConfig(env *config.CustomizeEnv) {
	if env.IsEmpty() {
		return
	}
	if adjusted := env.VPCConfig; adjusted != nil {
		cfg.loadAdjustedVPCConfig(adjusted)
	}
	if imported := env.ImportVPC; imported != nil {
		cfg.loadImportedVPCConfig(imported)
	}
}

func (cfg *environmentVPCConfig) loadAdjustedVPCConfig(vpc *config.AdjustVPC) {
	cfg.CIDR = ipNetP(vpc.CIDR)
	cfg.Subnets.Public = make([]subnetConfiguration, len(vpc.PublicSubnetCIDRs))
	cfg.Subnets.Private = make([]subnetConfiguration, len(vpc.PrivateSubnetCIDRs))
	for i, cidr := range vpc.PublicSubnetCIDRs {
		cfg.Subnets.Public[i].CIDR = ipNetP(cidr)
		if len(vpc.AZs) > i {
			cfg.Subnets.Public[i].AZ = stringP(vpc.AZs[i])
		}
	}
	for i, cidr := range vpc.PrivateSubnetCIDRs {
		cfg.Subnets.Private[i].CIDR = ipNetP(cidr)
		if len(vpc.AZs) > i {
			cfg.Subnets.Private[i].AZ = stringP(vpc.AZs[i])
		}
	}
}

func (cfg *environmentVPCConfig) loadImportedVPCConfig(vpc *config.ImportVPC) {
	cfg.ID = stringP(vpc.ID)
	cfg.Subnets.Public = make([]subnetConfiguration, len(vpc.PublicSubnetIDs))
	for i, subnet := range vpc.PublicSubnetIDs {
		cfg.Subnets.Public[i].SubnetID = stringP(subnet)
	}
	cfg.Subnets.Private = make([]subnetConfiguration, len(vpc.PrivateSubnetIDs))
	for i, subnet := range vpc.PrivateSubnetIDs {
		cfg.Subnets.Private[i].SubnetID = stringP(subnet)
	}
}

// UnmarshalEnvironment deserializes the YAML input stream into an environment manifest object.
// If an error occurs during deserialization, then returns the error.
func UnmarshalEnvironment(in []byte) (*Environment, error) {
	var m Environment
	if err := yaml.Unmarshal(in, &m); err != nil {
		return nil, fmt.Errorf("unmarshal environment manifest: %w", err)
	}
	return &m, nil
}

func (cfg *environmentVPCConfig) imported() bool {
	return aws.StringValue(cfg.ID) != ""
}

func (cfg *environmentVPCConfig) managedVPCCustomized() bool {
	return aws.StringValue((*string)(cfg.CIDR)) != ""
}

// ImportedVPC returns configurations that import VPC resources if there is any.
func (cfg *environmentVPCConfig) ImportedVPC() *template.ImportVPC {
	if !cfg.imported() {
		return nil
	}
	var publicSubnetIDs, privateSubnetIDs []string
	for _, subnet := range cfg.Subnets.Public {
		publicSubnetIDs = append(publicSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	for _, subnet := range cfg.Subnets.Private {
		privateSubnetIDs = append(privateSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	return &template.ImportVPC{
		ID:               aws.StringValue(cfg.ID),
		PublicSubnetIDs:  publicSubnetIDs,
		PrivateSubnetIDs: privateSubnetIDs,
	}
}

// ManagedVPC returns configurations that configure VPC resources if there is any.
func (cfg *environmentVPCConfig) ManagedVPC() *template.ManagedVPC {
	// NOTE: In a managed VPC, #pub = #priv = #az.
	// Either the VPC isn't configured, or everything need to be explicitly configured.
	if !cfg.managedVPCCustomized() {
		return nil
	}
	publicSubnetCIDRs := make([]string, len(cfg.Subnets.Public))
	privateSubnetCIDRs := make([]string, len(cfg.Subnets.Public))
	azs := make([]string, len(cfg.Subnets.Public))

	// NOTE: sort based on `az`s to preserve the mappings between azs and public subnets, private subnets.
	// For example, if we have two subnets defined: public-subnet-1 ~ us-east-1a, and private-subnet-1 ~ us-east-1a.
	// We want to make sure that public-subnet-1, us-east-1a and private-subnet-1 are all at index 0 of in perspective lists.
	sort.Slice(cfg.Subnets.Public, func(i, j int) bool {
		return aws.StringValue(cfg.Subnets.Public[i].AZ) < aws.StringValue(cfg.Subnets.Public[j].AZ)
	})
	sort.Slice(cfg.Subnets.Private, func(i, j int) bool {
		return aws.StringValue(cfg.Subnets.Private[i].AZ) < aws.StringValue(cfg.Subnets.Private[j].AZ)
	})
	for idx, subnet := range cfg.Subnets.Public {
		publicSubnetCIDRs[idx] = aws.StringValue((*string)(subnet.CIDR))
		privateSubnetCIDRs[idx] = aws.StringValue((*string)(cfg.Subnets.Private[idx].CIDR))
		azs[idx] = aws.StringValue(subnet.AZ)
	}
	return &template.ManagedVPC{
		CIDR:               aws.StringValue((*string)(cfg.CIDR)),
		AZs:                azs,
		PublicSubnetCIDRs:  publicSubnetCIDRs,
		PrivateSubnetCIDRs: privateSubnetCIDRs,
	}
}

type subnetsConfiguration struct {
	Public  []subnetConfiguration `yaml:"public,omitempty"`
	Private []subnetConfiguration `yaml:"private,omitempty"`
}

// IsEmpty returns true if neither public subnets nor private subnets are configured.
func (cs subnetsConfiguration) IsEmpty() bool {
	return len(cs.Public) == 0 && len(cs.Private) == 0
}

type subnetConfiguration struct {
	SubnetID *string `yaml:"id"`
	CIDR     *IPNet  `yaml:"cidr"`
	AZ       *string `yaml:"az"`
}

type environmentObservability struct {
	ContainerInsights *bool `yaml:"container_insights,omitempty"`
}

// IsEmpty returns true if there is no configuration to the environment's observability.
func (o *environmentObservability) IsEmpty() bool {
	return o == nil || o.ContainerInsights == nil
}

func (o *environmentObservability) loadObsConfig(tele *config.Telemetry) {
	if tele == nil {
		return
	}
	o.ContainerInsights = &tele.EnableContainerInsights
}

type environmentHTTPConfig struct {
	Public  publicHTTPConfig  `yaml:"public,omitempty"`
	Private privateHTTPConfig `yaml:"private,omitempty"`
}

// IsEmpty returns true if neither the public ALB nor the internal ALB is configured.
func (cfg environmentHTTPConfig) IsEmpty() bool {
	return cfg.Public.IsEmpty() && cfg.Private.IsEmpty()
}

func (cfg *environmentHTTPConfig) loadLBConfig(env *config.CustomizeEnv) {
	if env.IsEmpty() {
		return
	}
	if env.ImportVPC != nil && len(env.ImportVPC.PublicSubnetIDs) == 0 {
		cfg.Private.InternalALBSubnets = env.InternalALBSubnets
		cfg.Private.Certificates = env.ImportCertARNs
		return
	}
	cfg.Public.Certificates = env.ImportCertARNs
}

type publicHTTPConfig struct {
	Certificates []string `yaml:"certificates,omitempty"`
}

// IsEmpty returns true if there are customization to the public ALB.
func (c publicHTTPConfig) IsEmpty() bool {
	return len(c.Certificates) == 0
}

type privateHTTPConfig struct {
	InternalALBSubnets []string `yaml:"subnets,omitempty"`
	Certificates       []string `yaml:"certificates,omitempty"`
}

// IsEmpty returns true if there are customization to the internal ALB.
func (c privateHTTPConfig) IsEmpty() bool {
	return len(c.InternalALBSubnets) == 0 && len(c.Certificates) == 0
}
