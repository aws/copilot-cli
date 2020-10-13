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
	var b bytes.Buffer
	if err := runner.Run("git", []string{"describe", "--always"}, command.Stdout(&b)); err != nil {
		return "", err
	}

	// NOTE: `git describe` output bytes includes a `\n` character, so we trim it out.
	return strings.TrimSpace(b.String()), nil
}

func askImageTag(tag string, prompter prompter, cmd runner) (string, error) {
	var err error
	if tag == "" {
		tag, err = getVersionTag(cmd)
		log.Warningln("Failed to default tag, are you in a git repository?")
		if err != nil {
			// User is not in a Git repository, so prompt for a tag.
			tag, err = prompter.Get(inputImageTagPrompt, "", prompt.RequireNonEmpty)
			if err != nil {
				return "", fmt.Errorf("prompt get image tag: %w", err)
			}
		}
	}
	return tag, err
}
