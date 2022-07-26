//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"fmt"
	"strings"
)

// ValidateBinary validates if the ssm plugin exists and needs update.
func (s SSMPluginCommand) ValidateBinary() error {
	var latestVersion, currentVersion string
	if err := s.runner.Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, Stdout(&s.latestVersionBuffer)); err != nil {
		return fmt.Errorf("get ssm plugin latest version: %w", err)
	}
	latestVersion = strings.TrimSpace(s.latestVersionBuffer.String())
	if err := s.runner.Run(ssmPluginBinaryName, []string{"--version"}, Stdout(&s.currentVersionBuffer)); err != nil {
		if !strings.Contains(err.Error(), executableNotExistErrMessage) {
			return fmt.Errorf("get local ssm plugin version: %w", err)
		}
		return &ErrSSMPluginNotExist{}
	}
	currentVersion = strings.TrimSpace(s.currentVersionBuffer.String())
	if currentVersion != latestVersion {
		return &ErrOutdatedSSMPlugin{
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
		}
	}
	return nil
}
