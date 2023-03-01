// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	isUbuntu, err := s.isUbuntu()
	if err != nil {
		return err
	}
	if isUbuntu {
		return s.installUbuntuBinary()
	}
	return s.installLinuxBinary()
}

func (s SSMPluginCommand) isUbuntu() (bool, error) {
	if err := s.runner.Run("uname", []string{"-a"}, Stdout(&s.linuxDistVersionBuffer)); err != nil {
		return false, fmt.Errorf("get linux distribution version: %w", err)
	}
	return strings.Contains(s.linuxDistVersionBuffer.String(), "Ubuntu"), nil
}

func (s SSMPluginCommand) installLinuxBinary() error {
	if err := download(s.http, filepath.Join(s.tempDir, "session-manager-plugin.rpm"), linuxSSMPluginBinaryURL); err != nil {
		return fmt.Errorf("download ssm plugin: %w", err)
	}
	if err := s.runner.Run("sudo", []string{"yum", "install", "-y",
		filepath.Join(s.tempDir, "session-manager-plugin.rpm")}); err != nil {
		return err
	}
	return nil
}

func (s SSMPluginCommand) installUbuntuBinary() error {
	if err := download(s.http, filepath.Join(s.tempDir, "session-manager-plugin.deb"), ubuntuSSMPluginBinaryURL); err != nil {
		return fmt.Errorf("download ssm plugin: %w", err)
	}
	if err := s.runner.Run("sudo", []string{"dpkg", "-i",
		filepath.Join(s.tempDir, "session-manager-plugin.deb")}); err != nil {
		return err
	}
	return nil
}
