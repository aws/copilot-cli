// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/spf13/cobra"
)

type listAppOpts struct {
	store applicationLister
	w     io.Writer
}

// Execute writes the existing applications.
func (o *listAppOpts) Execute() error {
	apps, err := o.store.ListApplications()
	if err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	for _, app := range apps {
		fmt.Fprintln(o.w, app.Name)
	}

	return nil
}

// BuildAppListCommand builds the command to list existing applications.
func BuildAppListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the applications in your account.",
		Example: `
  List all the applications in your account and region.
  /code $ copilot app ls`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts := listAppOpts{
				w: os.Stdout,
			}
			ssmStore, err := config.NewStore()
			if err != nil {
				return err
			}
			opts.store = ssmStore
			return opts.Execute()
		}),
	}
	return cmd
}
