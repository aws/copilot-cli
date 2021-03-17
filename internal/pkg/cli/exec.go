// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

const (
	defaultCommand = "/bin/sh"
)

type execVars struct {
	appName          string
	envName          string
	name             string
	command          string
	taskID           string
	containerName    string
	skipConfirmation *bool // If nil, we will prompt to upgrade the ssm plugin.
}
