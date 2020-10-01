// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the copilot subcommands.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

// tryReadingAppName retrieves the application's name from the workspace if it exists and returns it.
// If there is an error while retrieving the workspace summary, returns the empty string.
func tryReadingAppName() string {
	ws, err := workspace.New()
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

// relPath returns the path relative to the current working directory.
func relPath(fullPath string) (string, error) {
	wkdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	path, err := filepath.Rel(wkdir, fullPath)
	if err != nil {
		return "", fmt.Errorf("get relative path of file: %w", err)
	}
	return path, nil
}

// relativeDockerfilePath returns the path from the workspace root to the Dockerfile.
func relativeDockerfilePath(ws copilotDirGetter, dockerfilePath string) (string, error) {
	copilotDirPath, err := ws.CopilotDirPath()
	if err != nil {
		return "", fmt.Errorf("get copilot directory: %w", err)
	}
	wsRoot := filepath.Dir(copilotDirPath)
	absDfPath, err := filepath.Abs(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	relDfPath, err := filepath.Rel(wsRoot, absDfPath)
	if err != nil {
		return "", fmt.Errorf("find relative path from workspace root to Dockerfile: %v", err)
	}
	return relDfPath, nil
}

// dfBuildRequired returns if the container image should be built from local Dockerfile.
func dfBuildRequired(svc interface{}) (bool, error) {
	type manifest interface {
		BuildRequired() (bool, error)
	}
	mf, ok := svc.(manifest)
	if !ok {
		return false, fmt.Errorf("service does not have required methods BuildRequired()")
	}
	required, err := mf.BuildRequired()
	if err != nil {
		return false, fmt.Errorf("check if service requires building from local Dockerfile: %w", err)
	}
	return required, nil
}
