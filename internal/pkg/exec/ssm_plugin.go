// +build !darwin,!linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

const (
	ssmPluginBinaryName = "session-manager-plugin"
	startSessionAction  = "StartSession"
)

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
	sess *session.Session
	runner
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

// ValidateBinary validates if the ssm plugin exists.
func (s SSMPluginCommand) ValidateBinary() error {
	// Hinder output on the screen.
	var b bytes.Buffer
	return s.runner.Run(ssmPluginBinaryName, []string{}, command.Stdout(&b))
}

// InstallLatestBinary returns nil and ssm plugin needs to be installed manually.
func (s SSMPluginCommand) InstallLatestBinary() error {
	return nil
}
