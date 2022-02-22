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
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcStatusNamePrompt     = "Which service's status would you like to show?"
	svcStatusNameHelpPrompt = "Displays the service's task status, most recent deployment and alarm statuses."
)

type svcStatusVars struct {
	shouldOutputJSON bool
	svcName          string
	envName          string
	appName          string
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
	defaultSess, err := sessions.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	return &svcStatusOpts{
		svcStatusVars: vars,
		store:         configStore,
		w:             log.OutputWriter,
		sel:           selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		initStatusDescriber: func(o *svcStatusOpts) error {
			wkld, err := configStore.GetWorkload(o.appName, o.svcName)
			if err != nil {
				return fmt.Errorf("retrieve %s from application %s: %w", o.appName, o.svcName, err)
			}
			if wkld.Type == manifest.RequestDrivenWebServiceType {
				d, err := describe.NewAppRunnerStatusDescriber(&describe.NewServiceStatusConfig{
					App:         o.appName,
					Env:         o.envName,
					Svc:         o.svcName,
					ConfigStore: configStore,
				})
				if err != nil {
					return fmt.Errorf("creating status describer for apprunner service %s in application %s: %w", o.svcName, o.appName, err)
				}
				o.statusDescriber = d
			} else {
				d, err := describe.NewECSStatusDescriber(&describe.NewServiceStatusConfig{
					App:         o.appName,
					Env:         o.envName,
					Svc:         o.svcName,
					ConfigStore: configStore,
				})
				if err != nil {
					return fmt.Errorf("creating status describer for service %s in application %s: %w", o.svcName, o.appName, err)
				}
				o.statusDescriber = d
			}
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *svcStatusOpts) Validate() error {
	if o.appName == "" {
		return nil
	}
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return err
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
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
		fmt.Fprint(o.w, data)
	} else {
		fmt.Fprint(o.w, svcStatus.HumanString())
	}

	return nil
}

func (o *svcStatusOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(svcAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcStatusOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, o.appName, selector.WithEnv(o.envName), selector.WithSvc(o.svcName))
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.appName, err)
	}
	o.svcName = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

// buildSvcStatusCmd builds the command for showing the status of a deployed service.
func buildSvcStatusCmd() *cobra.Command {
	vars := svcStatusVars{}
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	return cmd
}
