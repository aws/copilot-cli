// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

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
	initDescriber func() error // Overridden in tests.

	// Cached variables.
	targetSvc *config.Workload
}

func newShowSvcOpts(vars showSvcVars) (*showSvcOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc show"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	ssmStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, ssmStore)
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
		svc, err := opts.getTargetSvc()
		if err != nil {
			return err
		}
		switch svc.Type {
		case manifest.LoadBalancedWebServiceType:
			d, err = describe.NewLBWebServiceDescriber(describe.NewServiceConfig{
				App:             opts.appName,
				Svc:             opts.svcName,
				ConfigStore:     ssmStore,
				DeployStore:     deployStore,
				EnableResources: opts.shouldOutputResources,
			})
		case manifest.RequestDrivenWebServiceType:
			d, err = describe.NewRDWebServiceDescriber(describe.NewServiceConfig{
				App:             opts.appName,
				Svc:             opts.svcName,
				ConfigStore:     ssmStore,
				DeployStore:     deployStore,
				EnableResources: opts.shouldOutputResources,
			})
		case manifest.BackendServiceType:
			d, err = describe.NewBackendServiceDescriber(describe.NewServiceConfig{
				App:             opts.appName,
				Svc:             opts.svcName,
				ConfigStore:     ssmStore,
				DeployStore:     deployStore,
				EnableResources: opts.shouldOutputResources,
			})
		case manifest.WorkerServiceType:
			d, err = describe.NewWorkerServiceDescriber(describe.NewServiceConfig{
				App:             opts.appName,
				Svc:             opts.svcName,
				ConfigStore:     ssmStore,
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

// Validate returns an error for any invalid optional flags.
func (o *showSvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *showSvcOpts) Ask() error {
	if err := o.validateOrAskApp(); err != nil {
		return err
	}
	return o.validateOrAskSvcName()
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

func (o *showSvcOpts) validateOrAskApp() error {
	if o.appName != "" {
		_, err := o.store.GetApplication(o.appName)
		return err
	}
	appName, err := o.sel.Application(svcAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = appName
	return nil
}

func (o *showSvcOpts) validateOrAskSvcName() error {
	if o.svcName != "" {
		_, err := o.getTargetSvc()
		return err
	}
	svcName, err := o.sel.Service(fmt.Sprintf(svcShowSvcNamePrompt, color.HighlightUserInput(o.appName)),
		svcShowSvcNameHelpPrompt, o.appName)
	if err != nil {
		return fmt.Errorf("select service for application %s: %w", o.appName, err)
	}
	o.svcName = svcName

	return nil
}

func (o *showSvcOpts) getTargetSvc() (*config.Workload, error) {
	if o.targetSvc != nil {
		return o.targetSvc, nil
	}
	svc, err := o.store.GetService(o.appName, o.svcName)
	if err != nil {
		return nil, err
	}
	o.targetSvc = svc
	return o.targetSvc, nil
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, svcResourcesFlagDescription)
	return cmd
}
