// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
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
	ImportVpcConfig          *ImportVpcConfig
	AdjustVpcConfig          *AdjustVpcConfig
}

// ImportVpcOpts converts the environment's vpc importing configuration into a format parsable by the templates pkg.
func (e CreateEnvironmentInput) ImportVpcOpts() *template.ImportVpcOpts {
	if e.ImportVpcConfig == nil {
		return nil
	}
	return &template.ImportVpcOpts{
		ID:               e.ImportVpcConfig.ID,
		PrivateSubnetIDs: e.ImportVpcConfig.PrivateSubnetIDs,
		PublicSubnetIDs:  e.ImportVpcConfig.PublicSubnetIDs,
	}
}

// AdjustVpcOpts converts the environment's vpc adjusting configuration into a format parsable by the templates pkg.
func (e CreateEnvironmentInput) AdjustVpcOpts() *template.AdjustVpcOpts {
	if e.AdjustVpcConfig == nil {
		return nil
	}
	return &template.AdjustVpcOpts{
		CIDR:               e.AdjustVpcConfig.CIDR,
		PrivateSubnetCIDRs: e.AdjustVpcConfig.PrivateSubnetCIDRs,
		PublicSubnetCIDRs:  e.AdjustVpcConfig.PublicSubnetCIDRs,
	}
}

// ImportVpcConfig holds the fields to import VPC resources.
type ImportVpcConfig struct {
	ID               string // ID for the VPC.
	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
}

// AdjustVpcConfig holds the fields to adjust default VPC resources.
type AdjustVpcConfig struct {
	CIDR               string // CIDR range for the VPC.
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
