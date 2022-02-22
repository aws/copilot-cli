// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

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
	svcPauseNamePrompt        = "Which service of %s would you like to pause?"
	svcPauseSvcNameHelpPrompt = "The selected service will be paused."

	fmtSvcPauseStart         = "Pausing service %s in environment %s."
	fmtsvcPauseFailed        = "Failed to pause service %s in environment %s.\n"
	fmtSvcPauseSucceed       = "Paused service %s in environment %s.\n"
	fmtSvcPauseConfirmPrompt = "Are you sure you want to stop processing requests for service %s?"
)

type svcPauseVars struct {
	svcName          string
	envName          string
	appName          string
	skipConfirmation bool
}

type svcPauseOpts struct {
	svcPauseVars
	store        store
	prompt       prompter
	sel          deploySelector
	client       servicePauser
	initSvcPause func() error
	svcARN       string
	prog         progress
}

func newSvcPauseOpts(vars svcPauseVars) (*svcPauseOpts, error) {
	sessProvider := sessions.NewProvider(sessions.UserAgentExtras("svc pause"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	prompter := prompt.New()
	opts := &svcPauseOpts{
		svcPauseVars: vars,
		store:        configStore,
		prompt:       prompter,
		sel:          selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		prog:         termprogress.NewSpinner(log.DiagnosticWriter),
	}
	opts.initSvcPause = func() error {
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
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
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
	if o.appName == "" {
		return nil
	}
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return err
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
	if err := o.askSvcEnvName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	pauseConfirmed, err := o.prompt.Confirm(fmt.Sprintf(fmtSvcPauseConfirmPrompt, color.HighlightUserInput(o.svcName)), "", prompt.WithConfirmFinalMessage())
	if err != nil {
		return fmt.Errorf("svc pause confirmation prompt: %w", err)
	}
	if !pauseConfirmed {
		return errors.New("svc pause cancelled - no changes made")
	}
	return nil
}

func (o *svcPauseOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(svcPauseAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcPauseOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(
		fmt.Sprintf(svcPauseNamePrompt, color.HighlightUserInput(o.appName)),
		svcPauseSvcNameHelpPrompt,
		o.appName,
		selector.WithEnv(o.envName),
		selector.WithSvc(o.svcName),
		selector.WithServiceTypesFilter([]string{manifest.RequestDrivenWebServiceType}),
	)
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
	o.prog.Start(fmt.Sprintf(fmtSvcPauseStart, o.svcName, o.envName))

	err := o.client.PauseService(o.svcARN)
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtsvcPauseFailed, o.svcName, o.envName))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtSvcPauseSucceed, o.svcName, o.envName))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *svcPauseOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Run %s to start processing requests again.", color.HighlightCode(fmt.Sprintf("copilot svc resume -n %s", o.svcName))),
	})
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
