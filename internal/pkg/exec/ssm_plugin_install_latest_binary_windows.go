// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

// InstallLatestBinary returns nil and ssm plugin needs to be installed manually.
func (s SSMPluginCommand) InstallLatestBinary() error {
	return nil
}
