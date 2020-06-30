// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

func getVersionTag(runner runner) (string, error) {
	var b bytes.Buffer

	if err := runner.Run("git", []string{"describe", "--always"}, command.Stdout(&b)); err != nil {
		return "", err
	}

	// NOTE: `git describe` output bytes includes a `\n` character, so we trim it out.
	return strings.TrimSpace(b.String()), nil
}
