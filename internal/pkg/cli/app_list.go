// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

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

// buildAppListCommand builds the command to list existing applications.
func buildAppListCommand() *cobra.Command {
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
			sess, err := sessions.ImmutableProvider(sessions.UserAgentExtras("app ls")).Default()
			if err != nil {
				return fmt.Errorf("default session: %v", err)
			}
			opts.store = config.NewSSMStore(identity.New(sess), ssm.New(sess), aws.StringValue(sess.Config.Region))
			return opts.Execute()
		}),
	}
	return cmd
}
