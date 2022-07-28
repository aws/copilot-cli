// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import "errors"

var (
	// ErrLocalEnvsNotFound is returned when there are no environment manifests in the workspace.
	ErrLocalEnvsNotFound = errors.New("no environments found")
	// ErrVPCNotFound is returned when no existing VPCs are found.
	ErrVPCNotFound = errors.New("no existing VPCs found")
	// ErrSubnetsNotFound is returned when no existing subnets are found.
	ErrSubnetsNotFound = errors.New("no existing subnets found")
)
