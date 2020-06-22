// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/selector"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	svcShowAppNamePrompt     = "Which application's service would you like to show?"
	svcShowAppNameHelpPrompt = "An application groups all of your services together."
	svcShowSvcNamePrompt     = "Which service of %s would you like to show?"
	svcShowSvcNameHelpPrompt = "The details of a service will be shown (e.g., endpoint URL, CPU, Memory)."
)

type showSvcVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	svcName               string
}

type showSvcOpts struct {
	showSvcVars

	w             io.Writer
	store         store
	describer     describer
	ws            wsSvcReader
	sel           configSelector
	initDescriber func(bool) error // Overriden in tests.
}

func newShowSvcOpts(vars showSvcVars) (*showSvcOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	opts := &showSvcOpts{
		showSvcVars: vars,
		store:       ssmStore,
		ws:          ws,
		w:           log.OutputWriter,
		sel:         selector.NewConfigSelect(vars.prompt, ssmStore),
	}
	opts.initDescriber = func(enableResources bool) error {
		var d describer
		svc, err := opts.store.GetService(opts.AppName(), opts.svcName)
		if err != nil {
			return err
		}
		switch svc.Type {
		case manifest.LoadBalancedWebServiceType:
			if enableResources {
				d, err = describe.NewWebServiceDescriberWithResources(opts.AppName(), opts.svcName)
			} else {
				d, err = describe.NewWebServiceDescriber(opts.AppName(), opts.svcName)
			}
		case manifest.BackendServiceType:
			if enableResources {
				d, err = describe.NewBackendServiceDescriberWithResources(opts.AppName(), opts.svcName)
			} else {
				d, err = describe.NewBackendServiceDescriber(opts.AppName(), opts.svcName)
			}
		default:
			return fmt.Errorf("invalid service type %s", svc.Type)
		}

		if err != nil {
			return fmt.Errorf("creating describer for service %s in application %s: %w", opts.svcName, opts.AppName(), err)
		}
		opts.describer = d
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showSvcOpts) Validate() error {
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
		// If there are no local services in the workspace, we exit without error.
		return nil
	}
	err := o.initDescriber(o.shouldOutputResources)
	if err != nil {
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
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, svc.HumanString())
	}

	return nil
}

func (o *showSvcOpts) askApp() error {
	if o.AppName() != "" {
		return nil
	}
	appName, err := o.sel.Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = appName

	return nil
}

func (o *showSvcOpts) askSvcName() error {
	// return if service name is set by flag
	if o.svcName != "" {
		return nil
	}

	svcNames, err := o.retrieveLocalService()
	if err != nil {
		svcNames, err = o.retrieveAllServices()
		if err != nil {
			return err
		}
	}

	if len(svcNames) == 0 {
		log.Infof("No services found in application %s.\n", color.HighlightUserInput(o.AppName()))
		return nil
	}
	if len(svcNames) == 1 {
		o.svcName = svcNames[0]
		return nil
	}
	svcName, err := o.prompt.SelectOne(
		fmt.Sprintf(svcShowSvcNamePrompt, color.HighlightUserInput(o.AppName())),
		svcShowSvcNameHelpPrompt,
		svcNames,
	)
	if err != nil {
		return fmt.Errorf("select service for application %s: %w", o.AppName(), err)
	}
	o.svcName = svcName

	return nil
}

func (o *showSvcOpts) retrieveLocalService() ([]string, error) {
	localSvcNames, err := o.ws.ServiceNames()
	if err != nil {
		return nil, err
	}
	if len(localSvcNames) == 0 {
		return nil, errors.New("no service found")
	}
	return localSvcNames, nil
}

func (o *showSvcOpts) retrieveAllServices() ([]string, error) {
	svcs, err := o.store.ListServices(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list services for application %s: %w", o.AppName(), err)
	}
	svcNames := make([]string, len(svcs))
	for ind, svc := range svcs {
		svcNames[ind] = svc.Name
	}

	return svcNames, nil
}

// BuildSvcShowCmd builds the command for showing services in an application.
func BuildSvcShowCmd() *cobra.Command {
	vars := showSvcVars{
		GlobalOpts: NewGlobalOpts(),
	}
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
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, svcResourcesFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	return cmd
}
