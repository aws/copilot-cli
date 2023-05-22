// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstallLatestBinary installs the latest ssm plugin.
func (s SSMPluginCommand) InstallLatestBinary() error {
	if s.tempDir == "" {
		dir, err := os.MkdirTemp("", "ssmplugin")
		if err != nil {
			return fmt.Errorf("create a temporary directory: %w", err)
		}
		defer os.RemoveAll(dir)
		s.tempDir = dir
	}
	if err := download(s.http, filepath.Join(s.tempDir, "sessionmanager-bundle.zip"), ssmPluginBinaryURL); err != nil {
		return fmt.Errorf("download ssm plugin: %w", err)
	}
	if err := s.runner.Run("unzip", []string{"-o", filepath.Join(s.tempDir, "sessionmanager-bundle.zip"),
		"-d", s.tempDir}); err != nil {
		return err
	}
	if err := s.runner.Run("sudo", []string{filepath.Join(s.tempDir, "sessionmanager-bundle", "install"), "-i",
		"/usr/local/sessionmanagerplugin", "-b",
		"/usr/local/bin/session-manager-plugin"}); err != nil {
		return err
	}
	return nil
}
