// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import "strings"

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
