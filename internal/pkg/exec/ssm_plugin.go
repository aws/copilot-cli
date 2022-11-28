// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	ssmPluginBinaryName             = "session-manager-plugin"
	startSessionAction              = "StartSession"
	executableNotExistErrMessage    = "executable file not found"
	ssmPluginBinaryLatestVersionURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/VERSION"
)

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
	sess *session.Session
	runner
	http httpClient

	// facilitate unit test.
	latestVersionBuffer    bytes.Buffer
	currentVersionBuffer   bytes.Buffer
	linuxDistVersionBuffer bytes.Buffer
	tempDir                string
}

// NewSSMPluginCommand returns a SSMPluginCommand.
func NewSSMPluginCommand(s *session.Session) SSMPluginCommand {
	return SSMPluginCommand{
		runner: NewCmd(),
		sess:   s,
		http:   http.DefaultClient,
	}
}

// StartSession starts a session using the ssm plugin.
func (s SSMPluginCommand) StartSession(ssmSess *ecs.Session) error {
	response, err := json.Marshal(ssmSess)
	if err != nil {
		return fmt.Errorf("marshal session response: %w", err)
	}
	if err := s.runner.InteractiveRun(ssmPluginBinaryName,
		[]string{string(response), aws.StringValue(s.sess.Config.Region), startSessionAction}); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	return nil
}

func download(client httpClient, filepath string, url string) error {
	resp, err := client.Get(url)
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
