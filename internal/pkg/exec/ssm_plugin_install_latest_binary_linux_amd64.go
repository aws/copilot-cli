// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	ssmPluginBinaryURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/linux_64bit/session-manager-plugin.rpm"
)

// InstallLatestBinary installs the latest ssm plugin.
func (s SSMPluginCommand) InstallLatestBinary() error {
	if s.tempDir == "" {
		dir, err := ioutil.TempDir("", "ssmplugin")
		if err != nil {
			return fmt.Errorf("create a temporary directory: %w", err)
		}
		defer os.RemoveAll(dir)
		s.tempDir = dir
	}
	if err := download(filepath.Join(s.tempDir, "session-manager-plugin.rpm"), ssmPluginBinaryURL); err != nil {
		return fmt.Errorf("download ssm plugin: %w", err)
	}
	if err := s.runner.Run("sudo", []string{"yum", "install", "-y",
		filepath.Join(s.tempDir, "session-manager-plugin.rpm")}); err != nil {
		return err
	}
	return nil
}
