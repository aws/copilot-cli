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
	DomainHostedZone      string            // Hosted Zone ID for the domain.
	AdditionalTags        map[string]string // AdditionalTags are labels applied to resources under the application.
}
