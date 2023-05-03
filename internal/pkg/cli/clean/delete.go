// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package clean

// NoOp does nothing.
type NoOp struct{}

// Clean returns nil.
func (*NoOp) Clean(_, _, _ string) error {
	return nil
}
