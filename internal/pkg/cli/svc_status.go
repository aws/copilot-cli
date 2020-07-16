// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcStatusAppNamePrompt     = "Which application is the service in?"
	svcStatusAppNameHelpPrompt = "An application groups all of your services together."
	svcStatusNamePrompt        = "Which service's status would you like to show?"
	svcStatusNameHelpPrompt    = "Displays the service's task status, most recent deployment and alarm statuses."
)

type svcStatusVars struct {
	*GlobalOpts
	shouldOutputJSON bool
	svcName          string
	envName          string
}

type svcStatusOpts struct {
	svcStatusVars

	w                   io.Writer
	store               store
	statusDescriber     statusDescriber
	sel                 deploySelector
	initStatusDescriber func(*svcStatusOpts) error
}

func newSvcStatusOpts(vars svcStatusVars) (*svcStatusOpts, error) {
	configStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	return &svcStatusOpts{
		svcStatusVars: vars,
		store:         configStore,
		w:             log.OutputWriter,
		sel:           selector.NewDeploySelect(vars.prompt, configStore, deployStore),
		initStatusDescriber: func(o *svcStatusOpts) error {
			d, err := describe.NewServiceStatus(&describe.NewServiceStatusConfig{
				App:         o.AppName(),
				Env:         o.envName,
				Svc:         o.svcName,
				ConfigStore: configStore,
			})
			if err != nil {
				return fmt.Errorf("creating status describer for service %s in application %s: %w", o.svcName, o.AppName(), err)
			}
			o.statusDescriber = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *svcStatusOpts) Validate() error {
	if o.AppName() != "" {
		if _, err := o.store.GetApplication(o.AppName()); err != nil {
			return err
		}
	}
	if o.svcName != "" {
		if _, err := o.store.GetService(o.AppName(), o.svcName); err != nil {
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
func (o *svcStatusOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcEnvName()
}

// Execute displays the status of the service.
func (o *svcStatusOpts) Execute() error {
	err := o.initStatusDescriber(o)
	if err != nil {
		return err
	}
	svcStatus, err := o.statusDescriber.Describe()
	if err != nil {
		return fmt.Errorf("describe status of service %s: %w", o.svcName, err)
	}
	if o.shouldOutputJSON {
		data, err := svcStatus.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, svcStatus.HumanString())
	}

	return nil
}

func (o *svcStatusOpts) askApp() error {
	if o.AppName() != "" {
		return nil
	}
	app, err := o.sel.Application(svcStatusAppNamePrompt, svcStatusAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcStatusOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, o.AppName(), selector.WithEnv(o.envName), selector.WithSvc(o.svcName))
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.AppName(), err)
	}
	o.svcName = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

// BuildSvcStatusCmd builds the command for showing the status of a deployed service.
func BuildSvcStatusCmd() *cobra.Command {
	vars := svcStatusVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Shows status of a deployed service.",
		Long:  "Shows status of a deployed service's task status, most recent deployment and alarm statuses.",

		Example: `
  Shows status of the deployed service "my-svc"
  /code $ copilot svc status -n my-svc`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcStatusOpts(vars)
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
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	return cmd
}
