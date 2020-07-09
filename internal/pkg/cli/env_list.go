// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/spf13/cobra"
)

const (
	envListAppNamePrompt = "Which application is the environment in?"
	envListAppNameHelper = "An application is a collection of related services."
)

type listEnvVars struct {
	*GlobalOpts
	ShouldOutputJSON bool
}

type listEnvOpts struct {
	listEnvVars
	store store
	sel   configSelector

	w io.Writer
}

func newListEnvOpts(vars listEnvVars) (*listEnvOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}

	return &listEnvOpts{
		listEnvVars: vars,
		store:       store,
		sel:         selector.NewConfigSelect(vars.prompt, store),
		w:           os.Stdout,
	}, nil
}

// Ask asks for fields that are required but not passed in.
func (o *listEnvOpts) Ask() error {
	if o.AppName() != "" {
		return nil
	}
	app, err := o.sel.Application(envListAppNamePrompt, envListAppNameHelper)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

// Execute lists the environments through the prompt.
func (o *listEnvOpts) Execute() error {
	// Ensure the application actually exists before we try to list its environments.
	if _, err := o.store.GetApplication(o.AppName()); err != nil {
		return err
	}

	envs, err := o.store.ListEnvironments(o.AppName())
	if err != nil {
		return err
	}

	var out string
	if o.ShouldOutputJSON {
		data, err := o.jsonOutput(envs)
		if err != nil {
			return err
		}
		out = data
	} else {
		out = o.humanOutput(envs)
	}
	fmt.Fprintf(o.w, out)

	return nil
}

func (o *listEnvOpts) humanOutput(envs []*config.Environment) string {
	b := &strings.Builder{}
	for _, env := range envs {
		if env.Prod {
			fmt.Fprintf(b, "%s (prod)\n", color.Prod(env.Name))
		} else {
			fmt.Fprintln(b, env.Name)
		}
	}
	return b.String()
}

func (o *listEnvOpts) jsonOutput(envs []*config.Environment) (string, error) {
	type serializedEnvs struct {
		Environments []*config.Environment `json:"environments"`
	}
	b, err := json.Marshal(serializedEnvs{Environments: envs})
	if err != nil {
		return "", fmt.Errorf("marshal environments: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// BuildEnvListCmd builds the command for listing environments in an application.
func BuildEnvListCmd() *cobra.Command {
	vars := listEnvVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the environments in an application.",
		Example: `
  Lists all the environments for the frontend application.
  /code $ copilot env ls -a frontend`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newListEnvOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().BoolVar(&vars.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	return cmd
}
