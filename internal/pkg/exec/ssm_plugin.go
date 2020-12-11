// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

const (
	ssmPluginBinaryName             = "session-manager-plugin"
	ssmPluginBinaryLatestVersionURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/VERSION"
	ssmPluginBinaryMacOSURL         = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac/sessionmanager-bundle.zip"
	startSessionAction              = "StartSession"
	executableNotExistErrMessage    = "executable file not found"

	ssmPluginInstallPrompt = `Looks like your Session Manager plugin is not installed yet.
  Would you like to install the plugin to execute into the container?`
	ssmPluginInstallPromptHelp = `You must install the Session Manager plugin on your local machine to be able to execute into the container
  See https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html`
	ssmPluginUpdatePrompt = `Looks like your Session Manager plugin is using version %s.
  Would you like to update it to the latest version %s?`
)

var (
	errSSMPluginCommandInstallCancelled = errors.New("ssm plugin install cancelled")
	errSSMPluginCommandUpdateCancelled  = errors.New("ssm plugin update cancelled")
)

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
	sess     *session.Session
	prompter prompter
	runner

	// facilitate unit test.
	latestVersionBuffer  bytes.Buffer
	currentVersionBuffer bytes.Buffer
	tempDir              string
}

// NewSSMPluginCommand returns a SSMPluginCommand.
func NewSSMPluginCommand(s *session.Session) SSMPluginCommand {
	return SSMPluginCommand{
		runner:   command.New(),
		sess:     s,
		prompter: prompt.New(),
	}
}

// StartSession starts a session using the ssm plugin. And prompt to install the plugin
// if it doesn't exist.
func (s SSMPluginCommand) StartSession(ssmSess *ecs.Session) error {
	if err := s.validateBinary(); err != nil {
		return err
	}
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

func (s SSMPluginCommand) validateBinary() error {
	var latestVersion, currentVersion string
	if err := s.runner.Run("curl", []string{"-s", ssmPluginBinaryLatestVersionURL}, command.Stdout(&s.latestVersionBuffer)); err != nil {
		return fmt.Errorf("get ssm plugin latest version: %w", err)
	}
	latestVersion = strings.TrimSpace(s.latestVersionBuffer.String())
	if err := s.runner.Run(ssmPluginBinaryName, []string{"--version"}, command.Stdout(&s.currentVersionBuffer)); err != nil {
		if !strings.Contains(err.Error(), executableNotExistErrMessage) {
			return fmt.Errorf("get local ssm plugin version: %w", err)
		}
		// If ssm plugin is not install, prompt users to install the plugin.
		confirmInstall, err := s.prompter.Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp)
		if err != nil {
			return fmt.Errorf("prompt to confirm installing the plugin: %w", err)
		}
		if !confirmInstall {
			return errSSMPluginCommandInstallCancelled
		}
		if err := s.installBinary(latestVersion); err != nil {
			return fmt.Errorf("install binary: %w", err)
		}
		currentVersion = latestVersion
	} else {
		currentVersion = strings.TrimSpace(s.currentVersionBuffer.String())
	}
	if currentVersion == latestVersion {
		return nil
	}
	// If ssm plugin is not up to date, prompt users to update the plugin.
	confirmUpdate, err := s.prompter.Confirm(
		fmt.Sprintf(ssmPluginUpdatePrompt, currentVersion, latestVersion), "")
	if err != nil {
		return fmt.Errorf("prompt to confirm updating the plugin: %w", err)
	}
	if !confirmUpdate {
		return errSSMPluginCommandUpdateCancelled
	}
	if err := s.installBinary(latestVersion); err != nil {
		return fmt.Errorf("update binary: %w", err)
	}
	return nil
}

func (s SSMPluginCommand) installBinary(version string) error {
	if s.tempDir == "" {
		dir, err := ioutil.TempDir("", "temp")
		if err != nil {
			return fmt.Errorf("create a temporary directory: %w", err)
		}
		defer os.RemoveAll(dir)
		s.tempDir = dir
	}
	if err := download(filepath.Join(s.tempDir, "sessionmanager-bundle.zip"), ssmPluginBinaryMacOSURL); err != nil {
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
