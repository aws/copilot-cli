// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// package clean provides structs that clean resources deployed
// by Copilot. It is used prior to deleting a workload or environment
// so that the corresponding CloudFormation stack delete runs successfully.
package clean

// NoOp does nothing.
type NoOp struct{}

// Clean returns nil.
func (*NoOp) Clean() error {
	return nil
}
