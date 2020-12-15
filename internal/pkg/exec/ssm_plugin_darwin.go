// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

const (
	ssmPluginBinaryName             = "session-manager-plugin"
	ssmPluginBinaryLatestVersionURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/VERSION"
	ssmPluginBinaryURL              = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac/sessionmanager-bundle.zip"
	startSessionAction              = "StartSession"
	executableNotExistErrMessage    = "executable file not found"
)

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
	sess *session.Session
	runner

	// facilitate unit test.
	latestVersionBuffer  bytes.Buffer
	currentVersionBuffer bytes.Buffer
	tempDir              string
}

// NewSSMPluginCommand returns a SSMPluginCommand.
func NewSSMPluginCommand(s *session.Session) SSMPluginCommand {
	return SSMPluginCommand{
		runner: command.New(),
		sess:   s,
	}
}

// StartSession starts a session using the ssm plugin.
func (s SSMPluginCommand) StartSession(ssmSess *ecs.Session) error {
	response, err := json.Marshal(ssmSess)
	if err != nil {
		return fmt.Errorf("marshal session response: %w", err)
	}
	if err := s.runner.Run(ssmPluginBinaryName,
		[]string{string(response), *s.sess.Config.Region, startSessionAction},
		command.Stderr(os.Stderr), command.Stdin(os.Stdin), command.Stdout(os.Stdout)); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	return nil
}

// ValidateBinary validates if the ssm plugin exists and needs update.
func (s SSMPluginCommand) ValidateBinary() error {
	var latestVersion, currentVersion string
	if err := s.runner.Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, command.Stdout(&s.latestVersionBuffer)); err != nil {
		return fmt.Errorf("get ssm plugin latest version: %w", err)
	}
	latestVersion = strings.TrimSpace(s.latestVersionBuffer.String())
	if err := s.runner.Run(ssmPluginBinaryName, []string{"--version"}, command.Stdout(&s.currentVersionBuffer)); err != nil {
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
	if err := download(filepath.Join(s.tempDir, "sessionmanager-bundle.zip"), ssmPluginBinaryURL); err != nil {
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

func download(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
