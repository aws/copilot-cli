// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	// LegacyEnvTemplateVersion is the version associated with the environment template before we started versioning.
	LegacyEnvTemplateVersion = "v0.0.0"
	// LatestEnvTemplateVersion is the latest version number available for environment templates.
	LatestEnvTemplateVersion = "v1.5.0"
)

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	// The version of the environment template to create the stack. If empty, creates the legacy stack.
	Version string

	AppName                  string            // Name of the application this environment belongs to.
	Name                     string            // Name of the environment, must be unique within an application.
	Prod                     bool              // Whether or not this environment is a production environment.
	ToolsAccountPrincipalARN string            // The Principal ARN of the tools account.
	AppDNSName               string            // The DNS name of this application, if it exists
	AdditionalTags           map[string]string // AdditionalTags are labels applied to resources under the application.
	CustomResourcesURLs      map[string]string // Environment custom resource script S3 object URLs.
	ImportVPCConfig          *config.ImportVPC // Optional configuration if users have an existing VPC.
	AdjustVPCConfig          *config.AdjustVPC // Optional configuration if users want to override default VPC configuration.

	CFNServiceRoleARN string // Optional. A service role ARN that CloudFormation should use to make calls to resources in the stack.
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
