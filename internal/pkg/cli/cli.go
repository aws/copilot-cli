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
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GlobalOpts holds fields that are used across multiple commands.
type GlobalOpts struct {
	appName string
	prompt  prompter
}

// NewGlobalOpts returns a GlobalOpts with the application name retrieved from viper.
func NewGlobalOpts() *GlobalOpts {
	bindAppName()

	return &GlobalOpts{
		// Leave the appName as empty in case it's overwritten by a global flag.
		// See https://github.com/aws/copilot-cli/issues/570#issuecomment-569133741
		prompt: prompt.New(),
	}
}

// AppName returns the application name.
// If the name is empty, it caches it after querying viper.
func (o *GlobalOpts) AppName() string {
	if o.appName != "" {
		return o.appName
	}
	o.appName = viper.GetString(appFlag)
	return o.appName
}

// bindAppName loads the application's name to viper.
// If there is an error, we swallow the error and leave the default value as empty string.
func bindAppName() {
	name, err := loadAppName()
	if err != nil {
		return
	}
	viper.SetDefault(appFlag, name)
}

// loadAppName retrieves the application's name from the workspace if it exists and returns it.
// If there is an error, it returns an empty string and the error.
func loadAppName() (string, error) {
	ws, err := workspace.New()
	if err != nil {
		return "", fmt.Errorf("fetching workspace: %w", err)
	}

	summary, err := ws.Summary()
	if err != nil {
		return "", fmt.Errorf("reading from workspace: %w", err)
	}
	return summary.Application, nil
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
	for {
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
}

type svcEnv struct {
	svcName string
	envName string
}

func (s *svcEnv) String() string {
	return fmt.Sprintf("%s (%s)", s.svcName, s.envName)
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
