// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"strings"
	"unicode"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
)

const (
	dashReplacement = "DASH"
)

var templateFunctions = map[string]interface{}{
	"logicalIDSafe": logicalIDSafe,
}

// logicalIDSafe takes a CloudFormation logical ID, and
// sanitizes it by removing "-" characters (not allowed)
// and replacing them with "DASH" (allowed by CloudFormation but
// not permitted in ecs-cli generated resource names).
func logicalIDSafe(logicalID string) string {
	return strings.ReplaceAll(logicalID, "-", dashReplacement)
}

// safeLogicalIDToOriginal takes a "sanitized" logical ID
// and converts it back to its original form, with dashes.
func safeLogicalIDToOriginal(safeLogicalID string) string {
	return strings.ReplaceAll(safeLogicalID, dashReplacement, "-")
}

// fmtEnvVar transforms a CamelCase input string s into an upper SNAKE_CASE string and returns it.
// For example, "usersDdbTableName" becomes "USERS_DDB_TABLE_NAME".
func fmtEnvVar(s string) string {
	var name string
	for i, r := range s {
		if unicode.IsUpper(r) && i != 0 {
			name += "_"
		}
		name += string(unicode.ToUpper(r))
	}
	return name
}

func filterSecrets(outputs []addons.Output) []addons.Output {
	var secrets []addons.Output
	for _, out := range outputs {
		if out.IsSecret {
			secrets = append(secrets, out)
		}
	}
	return secrets
}

func filterManagedPolicies(outputs []addons.Output) []addons.Output {
	var policies []addons.Output
	for _, out := range outputs {
		if out.IsManagedPolicy {
			policies = append(policies, out)
		}
	}
	return policies
}
