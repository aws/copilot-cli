// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Environment holds the configuration to build an environment.
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

type subnetsConfiguration struct {
	Public  []subnetConfiguration `yaml:"public"`
	Private []subnetConfiguration `yaml:"private"`
}

type subnetConfiguration struct {
	SubnetID *string `yaml:"id"`
	CIDR     *IPNet  `yaml:"cidr"`
	AZ       *string `yaml:"az"`
}
