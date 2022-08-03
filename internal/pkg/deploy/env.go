// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

const (
	// LegacyEnvTemplateVersion is the version associated with the environment template before we started versioning.
	LegacyEnvTemplateVersion = "v0.0.0"
	// LatestEnvTemplateVersion is the latest version number available for environment templates.
	LatestEnvTemplateVersion = "v1.12.1"
)

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	// The version of the environment template to create the stack. If empty, creates the legacy stack.
	Version string

	// Application regional configurations.
	App                  AppInformation    // Information about the application that the environment belongs to, include app name, DNS name, the principal ARN of the account.
	Name                 string            // Name of the environment, must be unique within an application.
	AdditionalTags       map[string]string // AdditionalTags are labels applied to resources under the application.
	ArtifactBucketARN    string            // ARN of the regional application bucket.
	ArtifactBucketKeyARN string            // ARN of the KMS key used to encrypt the contents in the regional application bucket.

	// Runtime configurations.
	CustomResourcesURLs map[string]string //  Mapping of Custom Resource Function Name to the S3 URL where the function zip file is stored.

	// User inputs.
	ImportVPCConfig    *config.ImportVPC     // Optional configuration if users have an existing VPC.
	AdjustVPCConfig    *config.AdjustVPC     // Optional configuration if users want to override default VPC configuration.
	ImportCertARNs     []string              // Optional configuration if users want to import certificates.
	InternalALBSubnets []string              // Optional configuration if users want to specify internal ALB placement.
	AllowVPCIngress    bool                  // Optional configuration to allow access to internal ALB from ports 80/443.
	CIDRPrefixListIDs  []string              // Optional configuration to specify public security group ingress based on prefix lists.
	Telemetry          *config.Telemetry     // Optional observability and monitoring configuration.
	Mft                *manifest.Environment // Unmarshaled and interpolated manifest object.
	RawMft             []byte                // Content of the environment manifest without any modifications.
	ForceUpdate        bool

	CFNServiceRoleARN string // Optional. A service role ARN that CloudFormation should use to make calls to resources in the stack.
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
