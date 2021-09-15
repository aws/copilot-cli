// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

// ErrSSMPluginNotExist means the ssm plugin is not installed.
type ErrSSMPluginNotExist struct{}

func (e ErrSSMPluginNotExist) Error() string {
	return "Session Manager plugin does not exist"
}

// ErrOutdatedSSMPlugin means the ssm plugin is not up-to-date.
type ErrOutdatedSSMPlugin struct {
	CurrentVersion string
	LatestVersion  string
}

func (e ErrOutdatedSSMPlugin) Error() string {
	return "Session Manager plugin is not up-to-date"
}
