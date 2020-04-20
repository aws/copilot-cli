// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

// InstanceSummary represents the identifiers for a stack instance.
type InstanceSummary struct {
	StackID string
	Account string
	Region  string
}
