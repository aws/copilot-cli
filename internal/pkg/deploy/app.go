// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines application deployment resources.
package deploy

// CreateAppInput holds the fields required to create an application stack set.
type CreateAppInput struct {
	Name                  string            // Name of the application that needs to be created.
	AccountID             string            // AWS account ID to administrate the application.
	DNSDelegationAccounts []string          // Accounts to grant DNS access to for this application.
	DomainName            string            // DNS Name used for this application.
	DomainHostedZoneID    string            // Hosted Zone ID for the domain.
	AdditionalTags        map[string]string // AdditionalTags are labels applied to resources under the application.
	Version               string            // The version of the application template to create the stack/stackset. If empty, creates the legacy stack/stackset.
}

const (
	// LegacyAppTemplateVersion is the version associated with the application template before we started versioning.
	LegacyAppTemplateVersion = "v0.0.0"
	// LatestAppTemplateVersion is the latest version number available for application templates.
	LatestAppTemplateVersion = "v1.0.1"
)
