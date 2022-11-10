// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/cli/list"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type listWkldVars struct {
	appName                  string
	shouldOutputJSON         bool
	shouldShowLocalWorkloads bool
}

type listSvcOpts struct {
	listWkldVars

	// Interfaces to dependencies.
	sel  appSelector
	list workloadListWriter
}

func newListSvcOpts(vars listWkldVars) (*listSvcOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	sess, err := sessions.ImmutableProvider(sessions.UserAgentExtras("svc ls")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	store := config.NewSSMStore(identity.New(sess), ssm.New(sess), aws.StringValue(sess.Config.Region))
	svcLister := &list.SvcListWriter{
		Ws:    ws,
		Store: store,
		Out:   os.Stdout,

		ShowLocalSvcs: vars.shouldShowLocalWorkloads,
		OutputJSON:    vars.shouldOutputJSON,
	}

	return &listSvcOpts{
		listWkldVars: vars,

		list: svcLister,
		sel:  selector.NewAppEnvSelector(prompt.New(), store),
	}, nil
}

// Validate is a no-op for this command.
func (o *listSvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *listSvcOpts) Ask() error {
	if o.appName != "" {
		// NOTE: Skip validating app name here because `Execute` will fail pretty soon with a clear error message.
		// The validation (config.GetApplication) would only add additional operation time in this particular case.
		return nil
	}

	name, err := o.sel.Application(svcAppNamePrompt, wkldAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

// Execute lists the services through the prompt.
func (o *listSvcOpts) Execute() error {
	if err := o.list.Write(o.appName); err != nil {
		return err
	}

	return nil
}

// buildSvcListCmd builds the command for listing services in an appication.
func buildSvcListCmd() *cobra.Command {
	vars := listWkldVars{}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the services in an application.",
		Example: `
  Lists all the services for the "myapp" application.
  /code $ copilot svc ls --app myapp`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newListSvcOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldShowLocalWorkloads, localFlag, false, localSvcFlagDescription)
	return cmd
}
