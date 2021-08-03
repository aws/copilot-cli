// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
)

type shellCompleter interface {
	GenBashCompletion(w io.Writer) error
	GenZshCompletion(w io.Writer) error
	GenFishCompletion(w io.Writer, includeDesc bool) error
}

type completionOpts struct {
	Shell string // must be "bash" or "zsh" or "fish"

	w         io.Writer
	completer shellCompleter
}

// Validate returns an error if the shell is not "bash" or "zsh" or "fish".
func (opts *completionOpts) Validate() error {
	if opts.Shell == "bash" {
		return nil
	}
	if opts.Shell == "zsh" {
		return nil
	}
	if opts.Shell == "fish" {
		return nil
	}
	return errors.New("shell must be bash, zsh or fish")
}

// Execute writes the completion code to the writer.
// This method assumes that Validate() was called prior to invocation.
func (opts *completionOpts) Execute() error {
	if opts.Shell == "bash" {
		return opts.completer.GenBashCompletion(opts.w)
	}
	if opts.Shell == "zsh" {
		return opts.completer.GenZshCompletion(opts.w)
	}
	return opts.completer.GenFishCompletion(opts.w, true)
}

// BuildCompletionCmd returns the command to output shell completion code for the specified shell (bash or zsh or fish).
func BuildCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	opts := &completionOpts{}
	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Output shell completion code.",
		Long: `Output shell completion code for bash, zsh or fish.
The code must be evaluated to provide interactive completion of commands.`,
		Example: `
  Install zsh completion
  /code $ source <(copilot completion zsh)
  /code $ copilot completion zsh > "${fpath[1]}/_copilot" # to autoload on startup

  Install bash completion on macOS using homebrew
  /code $ brew install bash-completion   # if running 3.2
  /code $ brew install bash-completion@2 # if running Bash 4.1+
  /code $ copilot completion bash > /usr/local/etc/bash_completion.d

  Install bash completion on linux
  /code $ source <(copilot completion bash)
  /code $ copilot completion bash > copilot.sh
  /code $ sudo mv copilot.sh /etc/bash_completion.d/copilot

  Install fish completion
  /code$ copilot completion fish | source

  To load completions for each session, execute once:
  /code$ copilot completion fish > ~/.config/fish/completions/copilot.fish`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires a single shell argument (bash, zsh or fish)")
			}
			return nil
		},
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts.Shell = args[0]
			return opts.Validate()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts.w = os.Stdout
			opts.completer = rootCmd
			return opts.Execute()
		}),
	}
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Settings,
	}
	return cmd
}
