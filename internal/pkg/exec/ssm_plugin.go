// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

// SSMPlugin represents commands that can be run to trigger the ssm plugin.
type SSMPlugin struct {
	sess *session.Session
	runner
}

// NewSSMPlugin returns a SSMPlugin.
func NewSSMPlugin(s *session.Session) SSMPlugin {
	return SSMPlugin{
		runner: command.New(),
		sess:   s,
	}
}

// StartSession starts a session using the ssm plugin. And prompt to install the plugin
// if it doesn't exist.
func (s SSMPlugin) StartSession(ssmSess *ecs.Session) error {
	return nil
}
