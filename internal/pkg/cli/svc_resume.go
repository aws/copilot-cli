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
	svcResumeSvcNamePrompt     = "Which service of %s would you like to resume?"
	svcResumeSvcNameHelpPrompt = "The selected service will be resumed."

	fmtSvcResumeStarted = "Resuming service %s in environment %s."
	fmtSvcResumeFailed  = "Failed to resume service %s in environment %s: %v\n"
	fmtSvcResumeSuccess = "Resumed service %s in environment %s.\n"
)

type resumeSvcVars struct {
	appName string
	svcName string
	envName string
}

type resumeSvcInitClients func() error
type resumeSvcOpts struct {
	resumeSvcVars

	store              store
	serviceResumer     serviceResumer
	apprunnerDescriber apprunnerServiceDescriber
	spinner            progress
	sel                deploySelector
	initClients        resumeSvcInitClients
}

// Validate returns an error if the values provided by the user are invalid.
func (o *resumeSvcOpts) Validate() error {
	if o.appName == "" {
		return nil
	}
	if err := o.validateAppName(); err != nil {
		return err
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	if o.svcName != "" {
		if err := o.validateSvcName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *resumeSvcOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcEnvName()
}

// Execute resumes the service through the prompt.
func (o *resumeSvcOpts) Execute() error {
	if o.svcName == "" {
		return nil
	}
	if err := o.initClients(); err != nil {
		return err
	}
	svcARN, err := o.apprunnerDescriber.ServiceARN()
	if err != nil {
		return err
	}

	o.spinner.Start(fmt.Sprintf(fmtSvcResumeStarted, o.svcName, o.envName))
	if err := o.serviceResumer.ResumeService(svcARN); err != nil {
		o.spinner.Stop(log.Serrorf(fmtSvcResumeFailed, o.svcName, o.envName, err))
		return err
	}
	o.spinner.Stop(log.Ssuccessf(fmtSvcResumeSuccess, o.svcName, o.envName))
	return nil
}

func (o *resumeSvcOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return err
	}
	return nil
}

func (o *resumeSvcOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return err
	}
	return nil
}

func (o *resumeSvcOpts) validateSvcName() error {
	if _, err := o.store.GetService(o.appName, o.svcName); err != nil {
		return err
	}
	return nil
}

func (o *resumeSvcOpts) askApp() error {
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

func (o *resumeSvcOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(
		fmt.Sprintf(svcResumeSvcNamePrompt, color.HighlightUserInput(o.appName)),
		svcResumeSvcNameHelpPrompt,
		o.appName,
		selector.WithEnv(o.envName),
		selector.WithSvc(o.svcName),
		selector.WithServiceTypesFilter([]string{manifest.RequestDrivenWebServiceType}),
	)
	if err != nil {
		return fmt.Errorf("select deployed service for application %s: %w", o.appName, err)
	}
	o.svcName = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

func newResumeSvcOpts(vars resumeSvcVars) (*resumeSvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	deployStore, err := deploy.NewStore(store)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}

	opts := &resumeSvcOpts{
		resumeSvcVars: vars,
		store:         store,
		sel:           selector.NewDeploySelect(prompt.New(), store, deployStore),
		spinner:       termprogress.NewSpinner(log.DiagnosticWriter),
	}
	opts.initClients = func() error {
		var a *apprunner.AppRunner
		var d *describe.AppRunnerServiceDescriber
		configStore, err := config.NewStore()
		if err != nil {
			return err
		}
		env, err := configStore.GetEnvironment(opts.appName, opts.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		svc, err := opts.store.GetService(opts.appName, opts.svcName)
		if err != nil {
			return err
		}
		switch svc.Type {
		case manifest.RequestDrivenWebServiceType:
			sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return err
			}
			a = apprunner.New(sess)
			d, err = describe.NewAppRunnerServiceDescriber(describe.NewServiceConfig{
				App:         opts.appName,
				Env:         opts.envName,
				Svc:         opts.svcName,
				ConfigStore: configStore,
			})
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid service type %s", svc.Type)
		}

		if err != nil {
			return fmt.Errorf("creating describer for service %s in environment %s and application %s: %w", opts.svcName, opts.envName, opts.appName, err)
		}
		opts.serviceResumer = a
		opts.apprunnerDescriber = d
		return nil
	}
	return opts, nil
}

// buildSvcResumeCmd builds the command for resuming services in an application.
func buildSvcResumeCmd() *cobra.Command {
	vars := resumeSvcVars{}
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resumes a paused service.",
		Long:  "Resumes a paused service.",
		Example: `
  Resumes the service named "my-svc" in the "test" environment.
  /code $ copilot svc resume --name my-svc --env test`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newResumeSvcOpts(vars)
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
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	return cmd
}
