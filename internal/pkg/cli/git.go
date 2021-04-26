// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

func describeGitChanges(exec runner) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := exec.Run("git", []string{"describe", "--always"}, command.Stdout(&stdout), command.Stderr(&stderr)); err != nil {
		return "", err
	}
	// NOTE: `git describe` output bytes includes a `\n` character, so we trim it out.
	return strings.TrimSpace(stdout.String()), nil
}

func hasUncommitedGitChanges(exec runner) (bool, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := exec.Run("git", []string{"status", "--porcelain"}, command.Stdout(&stdout), command.Stderr(&stderr)); err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout.String()) != "", nil
}

// imageTagFromGit returns the image tag to apply in case the user is in a git repository.
// If the user provided their own tag, then just use that.
// If there is a clean git commit with no local changes, then return the git commit id.
// Otherwise, returns the empty string.
func imageTagFromGit(exec runner, userTag string) string {
	if userTag != "" {
		return userTag
	}
	commit, err := describeGitChanges(exec)
	if err != nil {
		return ""
	}
	isRepoDirty, _ := hasUncommitedGitChanges(exec)
	if isRepoDirty {
		return ""
	}
	return commit
}
