// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcPauseAppNamePrompt     = "Which application is the service in?"
	svcPauseAppNameHelpPrompt = "An application groups all of your services together."
	svcPauseNamePrompt        = "Which service would you like to pause?"

	fmtSvcPauseStart   = "Pausing App Runner service %s."
	fmtsvcPauseFailed  = "Failed to pause App Runner service %s."
	fmtSvcPauseSucceed = "Paused App Runner service %s."
)

type svcPauseVars struct {
	svcName string
	envName string
	appName string
}

type svcPauseOpts struct {
	svcPauseVars
	store        store
	sel          deploySelector
	client       servicePauser
	initSvcPause func() error
	svcARN       string
	prog         progress
}

func newSvcPauseOpts(vars svcPauseVars) (*svcPauseOpts, error) {
	configStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}

	opts := &svcPauseOpts{
		svcPauseVars: vars,
		store:        configStore,
		sel:          selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		prog:         termprogress.NewSpinner(log.DiagnosticWriter),
	}
	opts.initSvcPause = func() error {
		configStore, err := config.NewStore()
		if err != nil {
			return fmt.Errorf("connect to environment config store: %w", err)
		}
		env, err := configStore.GetEnvironment(opts.appName, opts.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		wl, err := configStore.GetWorkload(opts.appName, opts.svcName)
		if err != nil {
			return fmt.Errorf("get workload: %w", err)
		}
		if wl.Type != manifest.RequestDrivenWebServiceType {
			return fmt.Errorf("pausing a service is only supported for services with type: %s", manifest.RequestDrivenWebServiceType)
		}
		sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		opts.client = apprunner.New(sess)
		d, err := describe.NewAppRunnerServiceDescriber(describe.NewServiceConfig{
			App:         opts.appName,
			Env:         opts.envName,
			Svc:         opts.svcName,
			ConfigStore: opts.store,
		})
		if err != nil {
			return err
		}
		opts.svcARN, err = d.ServiceARN()
		if err != nil {
			return fmt.Errorf("retrieve ServiceARN for %s: %w", opts.svcName, err)
		}
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *svcPauseOpts) Validate() error {
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
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *svcPauseOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcEnvName()
}

func (o *svcPauseOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(svcPauseAppNamePrompt, svcPauseAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcPauseOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(svcPauseNamePrompt, "", o.appName, selector.WithEnv(o.envName), selector.WithSvc(o.svcName))
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.appName, err)
	}
	o.svcName = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

// Execute pause the running App Runner service.
func (o *svcPauseOpts) Execute() error {
	if err := o.initSvcPause(); err != nil {
		return err
	}

	log.Warningln("Your service will be unavailable while paused. You can resume the service once the pause operation is complete.")
	o.prog.Start(fmt.Sprintf(fmtSvcPauseStart, color.HighlightUserInput(o.svcName)))

	err := o.client.PauseService(o.svcARN)
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtsvcPauseFailed, color.HighlightUserInput(o.svcName)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtSvcPauseSucceed, color.HighlightUserInput(o.svcName)))
	return nil
}

// buildSvcPauseCmd builds the command for pausing the running service.
func buildSvcPauseCmd() *cobra.Command {
	vars := svcPauseVars{}
	cmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause running App Runner service.",
		Long:  "Pause running App Runner service.",

		Example: `
  Pause running App Runner service "my-svc".
  /code $ copilot svc pause -n my-svc`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcPauseOpts(vars)
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
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
