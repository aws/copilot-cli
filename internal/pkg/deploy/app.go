// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines application deployment resources.
package deploy

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/arn"
)

const appDNSDelegationRoleName = "DNSDelegationRole"

// CreateAppInput holds the fields required to create an application stack set.
type CreateAppInput struct {
	Name                  string            // Name of the application that needs to be created.
	AccountID             string            // AWS account ID to administrate the application.
	DNSDelegationAccounts []string          // Accounts to grant DNS access to for this application.
	DomainName            string            // DNS Name used for this application.
	DomainHostedZoneID    string            // Hosted Zone ID for the domain.
	PermissionsBoundary   string            // Name of the IAM Managed Policy to set a permissions boundary.
	AdditionalTags        map[string]string // AdditionalTags are labels applied to resources under the application.
	Version               string            // The version of the application template to create the stack/stackset. If empty, creates the legacy stack/stackset.
}

// AppInformation holds information about the application that need to be propagated to the env stacks and workload stacks.
type AppInformation struct {
	AccountPrincipalARN string
	Domain              string
	Name                string
	PermissionsBoundary string
}

// DNSDelegationRole returns the ARN of the app's DNS delegation role.
func (a *AppInformation) DNSDelegationRole() string {
	if a.AccountPrincipalARN == "" || a.Domain == "" {
		return ""
	}

	appRole, err := arn.Parse(a.AccountPrincipalARN)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("arn:%s:iam::%s:role/%s", appRole.Partition, appRole.AccountID, DNSDelegationRoleName(a.Name))
}

// DNSDelegationRoleName returns the DNSDelegation role name of the app.
func DNSDelegationRoleName(appName string) string {
	return fmt.Sprintf("%s-%s", appName, appDNSDelegationRoleName)
}
