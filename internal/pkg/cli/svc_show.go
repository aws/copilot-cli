// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcShowSvcNamePrompt     = "Which service of %s would you like to show?"
	svcShowSvcNameHelpPrompt = "The details of a service will be shown (e.g., endpoint URL, CPU, Memory)."
)

type showSvcVars struct {
	shouldOutputJSON      bool
	shouldOutputResources bool
	appName               string
	svcName               string
}

type showSvcOpts struct {
	showSvcVars

	w             io.Writer
	store         store
	describer     describer
	sel           configSelector
	initDescriber func() error // Overriden in tests.
}

func newShowSvcOpts(vars showSvcVars) (*showSvcOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	deployStore, err := deploy.NewStore(ssmStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}

	opts := &showSvcOpts{
		showSvcVars: vars,
		store:       ssmStore,
		w:           log.OutputWriter,
		sel:         selector.NewConfigSelect(prompt.New(), ssmStore),
	}
	opts.initDescriber = func() error {
		var d describer
		svc, err := opts.store.GetService(opts.appName, opts.svcName)
		if err != nil {
			return err
		}
		switch svc.Type {
		case manifest.LoadBalancedWebServiceType:
			d, err = describe.NewWebServiceDescriber(describe.NewWebServiceConfig{
				NewServiceConfig: describe.NewServiceConfig{
					App:         opts.appName,
					Svc:         opts.svcName,
					ConfigStore: ssmStore,
				},
				DeployStore:     deployStore,
				EnableResources: opts.shouldOutputResources,
			})
		case manifest.BackendServiceType:
			d, err = describe.NewBackendServiceDescriber(describe.NewBackendServiceConfig{
				NewServiceConfig: describe.NewServiceConfig{
					App:         opts.appName,
					Svc:         opts.svcName,
					ConfigStore: ssmStore,
				},
				DeployStore:     deployStore,
				EnableResources: opts.shouldOutputResources,
			})
		default:
			return fmt.Errorf("invalid service type %s", svc.Type)
		}

		if err != nil {
			return fmt.Errorf("creating describer for service %s in application %s: %w", opts.svcName, opts.appName, err)
		}
		opts.describer = d
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showSvcOpts) Validate() error {
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
	}
	if o.svcName != "" {
		if _, err := o.store.GetService(o.appName, o.svcName); err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showSvcOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcName()
}

// Execute shows the services through the prompt.
func (o *showSvcOpts) Execute() error {
	if o.svcName == "" {
		return nil
	}
	if err := o.initDescriber(); err != nil {
		return err
	}
	svc, err := o.describer.Describe()
	if err != nil {
		return fmt.Errorf("describe service %s: %w", o.svcName, err)
	}

	if o.shouldOutputJSON {
		data, err := svc.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprint(o.w, data)
	} else {
		fmt.Fprint(o.w, svc.HumanString())
	}

	return nil
}

func (o *showSvcOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	appName, err := o.sel.Application(svcAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = appName

	return nil
}

func (o *showSvcOpts) askSvcName() error {
	if o.svcName != "" {
		return nil
	}
	svcName, err := o.sel.Service(fmt.Sprintf(svcShowSvcNamePrompt, color.HighlightUserInput(o.appName)),
		svcShowSvcNameHelpPrompt, o.appName)
	if err != nil {
		return fmt.Errorf("select service for application %s: %w", o.appName, err)
	}
	o.svcName = svcName

	return nil
}

// buildSvcShowCmd builds the command for showing services in an application.
func buildSvcShowCmd() *cobra.Command {
	vars := showSvcVars{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a deployed service per environment.",
		Long:  "Shows info about a deployed service, including endpoints, capacity and related resources per environment.",

		Example: `
  Shows info about the service "my-svc"
  /code $ copilot svc show -n my-svc`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowSvcOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, svcResourcesFlagDescription)
	return cmd
}
