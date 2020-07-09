// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	envShowAppNamePrompt     = "Which application is the environment in?"
	envShowAppNameHelpPrompt = "An application is a collection of related services."
	envShowNamePrompt        = "Which environment of %s would you like to show?"
	envShowHelpPrompt        = "The detail of an environment will be shown (e.g., region, account ID, services)."
)

type showEnvVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	envName               string
}

type showEnvOpts struct {
	showEnvVars

	w                io.Writer
	store            store
	describer        envDescriber
	sel              configSelector
	initEnvDescriber func() error
}

func newShowEnvOpts(vars showEnvVars) (*showEnvOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to copilot config store: %w", err)
	}

	opts := &showEnvOpts{
		showEnvVars: vars,
		store:       store,
		w:           log.OutputWriter,
		sel:         selector.NewConfigSelect(vars.prompt, store),
	}
	opts.initEnvDescriber = func() error {
		var d envDescriber
		if !vars.shouldOutputResources {
			d, err = describe.NewEnvDescriber(opts.AppName(), opts.envName)
		} else {
			d, err = describe.NewEnvDescriberWithResources(opts.AppName(), opts.envName)
		}
		if err != nil {
			return fmt.Errorf("creating describer for environment %s in application %s: %w", opts.envName, opts.AppName(), err)
		}
		opts.describer = d
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showEnvOpts) Validate() error {
	if o.AppName() != "" {
		if _, err := o.store.GetApplication(o.AppName()); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.AppName(), o.envName); err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showEnvOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askEnvName()
}

// Execute shows the environments through the prompt.
func (o *showEnvOpts) Execute() error {
	if err := o.initEnvDescriber(); err != nil {
		return err
	}
	env, err := o.describer.Describe()
	if err != nil {
		return fmt.Errorf("describe environment %s: %w", o.envName, err)
	}
	if o.shouldOutputJSON {
		data, err := env.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, env.HumanString())
	}

	return nil
}

func (o *showEnvOpts) askApp() error {
	if o.AppName() != "" {
		return nil
	}
	app, err := o.sel.Application(envShowAppNamePrompt, envShowAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *showEnvOpts) askEnvName() error {
	//return if env name is set by flag
	if o.envName != "" {
		return nil
	}
	env, err := o.sel.Environment(fmt.Sprintf(envShowNamePrompt, color.HighlightUserInput(o.AppName())), envShowHelpPrompt, o.AppName())
	if err != nil {
		return fmt.Errorf("select environment for application %s: %w", o.AppName(), err)
	}
	o.envName = env

	return nil
}

// BuildEnvShowCmd builds the command for showing environments in an application.
func BuildEnvShowCmd() *cobra.Command {
	vars := showEnvVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: true, //TODO remove when ready for production!
		Use:    "show",
		Short:  "Shows info about a deployed environment.",
		Long:   "Shows info about a deployed environment, including region, account ID, and services.",

		Example: `
  Shows info about the environment "test".
  /code $ copilot env show -n test`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowEnvOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&vars.envName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, envResourcesFlagDescription)
	return cmd
}
