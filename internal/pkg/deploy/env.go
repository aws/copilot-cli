// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	// EmptyIPNetString is the return value of String() for an empty net.IPNet instance.
	EmptyIPNetString = "<nil>"
)

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	AppName                  string            // Name of the application this environment belongs to.
	Name                     string            // Name of the environment, must be unique within an application.
	Prod                     bool              // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool              // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string            // The Principal ARN of the tools account.
	AppDNSName               string            // The DNS name of this application, if it exists
	AdditionalTags           map[string]string // AdditionalTags are labels applied to resources under the application.
	ImportVPCConfig          *ImportVPCConfig
	AdjustVPCConfig          *AdjustVPCConfig
}

// ImportVPCOpts converts the environment's vpc importing configuration into a format parsable by the templates pkg.
func (e CreateEnvironmentInput) ImportVPCOpts() *template.ImportVPCOpts {
	if e.ImportVPCConfig == nil {
		return nil
	}
	return &template.ImportVPCOpts{
		ID:               e.ImportVPCConfig.ID,
		PrivateSubnetIDs: e.ImportVPCConfig.PrivateSubnetIDs,
		PublicSubnetIDs:  e.ImportVPCConfig.PublicSubnetIDs,
	}
}

// AdjustVPCOpts converts the environment's vpc adjusting configuration into a format parsable by the templates pkg.
func (e CreateEnvironmentInput) AdjustVPCOpts() *template.AdjustVPCOpts {
	if e.AdjustVPCConfig == nil {
		return nil
	}
	return &template.AdjustVPCOpts{
		CIDR:               e.AdjustVPCConfig.CIDR,
		PrivateSubnetCIDRs: e.AdjustVPCConfig.PrivateSubnetCIDRs,
		PublicSubnetCIDRs:  e.AdjustVPCConfig.PublicSubnetCIDRs,
	}
}

// ImportVPCConfig holds the fields to import VPC resources.
type ImportVPCConfig struct {
	ID               string // ID for the VPC.
	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
}

// IsEmpty returns true if ImportVPCConfig is empty.
func (c ImportVPCConfig) IsEmpty() bool {
	if (c.ID == "") && (len(c.PublicSubnetIDs) == 0) &&
		(len(c.PrivateSubnetIDs) == 0) {
		return true
	}
	return false
}

// AdjustVPCConfig holds the fields to adjust default VPC resources.
type AdjustVPCConfig struct {
	CIDR               string // CIDR range for the VPC.
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

// IsEmpty returns true if AdjustVPCConfig is empty.
func (c AdjustVPCConfig) IsEmpty() bool {
	if (c.CIDR == EmptyIPNetString) && (len(c.PublicSubnetCIDRs) == 0) &&
		(len(c.PrivateSubnetCIDRs) == 0) {
		return true
	}
	return false
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
