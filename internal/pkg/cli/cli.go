// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the copilot subcommands.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	svcAppNamePrompt      = "Which application does your service belong to?"
	wkldAppNameHelpPrompt = "An application groups all of your services and jobs together."
)

// tryReadingAppName retrieves the application's name from the workspace if it exists and returns it.
// If there is an error while retrieving the workspace summary, returns the empty string.
func tryReadingAppName() string {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return ""
	}

	summary, err := ws.Summary()
	if err != nil {
		return ""
	}
	return summary.Application
}

type errReservedArg struct {
	val string
}

func (e *errReservedArg) Error() string {
	return fmt.Sprintf(`argument %s is a reserved keyword, please use a different value`, color.HighlightUserInput(e.val))
}

// reservedArgs returns an error if the arguments contain any reserved keywords.
func reservedArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return nil
	}
	if args[0] == "local" {
		return &errReservedArg{val: "local"}
	}
	return nil
}

// runCmdE wraps one of the run error methods, PreRunE, RunE, of a cobra command so that if a user
// types "help" in the arguments the usage string is printed instead of running the command.
func runCmdE(f func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && args[0] == "help" {
			_ = cmd.Help() // Help always returns nil.
			os.Exit(0)
		}
		return f(cmd, args)
	}
}

// returns true if error type is stack set not exist.
func isStackSetNotExistsErr(err error) bool {
	if err == nil {
		return false
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return isStackSetNotExistsErr(errors.Unwrap(err))
	}
	if aerr.Code() != "StackSetNotFoundException" {
		return isStackSetNotExistsErr(errors.Unwrap(err))
	}
	return true
}

func run(cmd cmd) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	if err := cmd.Ask(); err != nil {
		return err
	}
	if err := cmd.Execute(); err != nil {
		return err
	}
	if actionCmd, ok := cmd.(actionCommand); ok {
		if err := actionCmd.RecommendActions(); err != nil {
			return err
		}
	}
	return nil
}

func logRecommendedActions(actions []string) {
	if len(actions) == 0 {
		return
	}
	log.Infoln(fmt.Sprintf("Recommended follow-up %s:", english.PluralWord(len(actions), "action", "actions")))
	for _, followup := range actions {
		log.Infof("%s\n", indentListItem(followup))
	}
}

func indentListItem(multiline string) string {
	var prefixedLines []string
	var inCodeBlock bool
	for i, line := range strings.Split(multiline, "\n") {
		if strings.Contains(line, "```") {
			inCodeBlock = !inCodeBlock
		}
		var prefix string
		switch {
		case i == 0:
			prefix = "  - "
		case inCodeBlock, strings.Contains(line, "```"):
			prefix = ""
		default:
			prefix = "    "
		}
		prefixedLines = append(prefixedLines, fmt.Sprintf("%s%s", prefix, line))
	}
	return strings.Join(prefixedLines, "\n")
}

func indentBy(multiline string, indentCount int) string {
	var prefixedLines []string
	for _, line := range strings.Split(multiline, "\n") {
		prefix := strings.Repeat(" ", indentCount)
		prefixedLines = append(prefixedLines, fmt.Sprintf("%s%s", prefix, line))
	}
	return strings.Join(prefixedLines, "\n")
}

func applyAll[T any](in []T, fn func(item T) T) []T {
	out := make([]T, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// displayPath takes any path and returns it in a form ready to be displayed to
// the user on the command line.
//
// No guarantees are given on the stability of the path across runs, all that is
// guaranteed is that the displayed path is visually pleasing & meaningful for a
// user.
//
// This path should not be stored in configuration files or used in any way except
// for being displayed to the user.
func displayPath(target string) string {
	if !filepath.IsAbs(target) {
		return filepath.Clean(target)
	}

	base, err := os.Getwd()
	if err != nil {
		return filepath.Clean(target)
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		// No path from base to target available, return target as is.
		return filepath.Clean(target)
	}
	return rel
}
