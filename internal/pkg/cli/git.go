// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

const (
	inputImageTagPrompt = "Input an image tag value:"
)

func getVersionTag(runner runner) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runner.Run("git", []string{"describe", "--always"}, command.Stdout(&stdout), command.Stderr(&stderr)); err != nil {
		return "", err
	}

	// NOTE: `git describe` output bytes includes a `\n` character, so we trim it out.
	return strings.TrimSpace(stdout.String()), nil
}

func askImageTag(tag string, prompter prompter, cmd runner) (string, error) {
	if tag != "" {
		return tag, nil
	}
	tag, err := getVersionTag(cmd)
	if err != nil {
		log.Warningln("Failed to default tag, are you in a git repository?")
		// User is not in a Git repository, so prompt for a tag.
		tag, err = prompter.Get(inputImageTagPrompt, "", prompt.RequireNonEmpty)
		if err != nil {
			return "", fmt.Errorf("prompt get image tag: %w", err)
		}
	}
	return tag, err
}
