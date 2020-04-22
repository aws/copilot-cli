// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"strings"

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

func secretOutputNames(outputs []addons.Output) []string {
	var secrets []string
	for _, out := range outputs {
		if out.IsSecret {
			secrets = append(secrets, out.Name)
		}
	}
	return secrets
}

func managedPolicyOutputNames(outputs []addons.Output) []string {
	var policies []string
	for _, out := range outputs {
		if out.IsManagedPolicy {
			policies = append(policies, out.Name)
		}
	}
	return policies
}

func envVarOutputNames(outputs []addons.Output) []string {
	var envVars []string
	for _, out := range outputs {
		if !out.IsSecret && !out.IsManagedPolicy {
			envVars = append(envVars, out.Name)
		}
	}
	return envVars
}
