// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
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

	// cached variables.
	targetEnv *config.Environment
}

func newSvcPauseOpts(vars svcPauseVars) (*svcPauseOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc pause"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, configStore)
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
		env, err := opts.getTargetEnv()
		if err != nil {
			return err
		}
		wl, err := configStore.GetWorkload(opts.appName, opts.svcName)
		if err != nil {
			return fmt.Errorf("get workload: %w", err)
		}
		if wl.Type != manifestinfo.RequestDrivenWebServiceType {
			return fmt.Errorf("pausing a service is only supported for services with type: %s", manifestinfo.RequestDrivenWebServiceType)
		}
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		opts.client = apprunner.New(sess)
		d, err := describe.NewRDWebServiceDescriber(describe.NewServiceConfig{
			App:         opts.appName,
			Svc:         opts.svcName,
			ConfigStore: opts.store,
		})
		if err != nil {
			return err
		}
		opts.svcARN, err = d.ServiceARN(opts.envName)
		if err != nil {
			return fmt.Errorf("retrieve ServiceARN for %s: %w", opts.svcName, err)
		}
		return nil
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *svcPauseOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *svcPauseOpts) Ask() error {
	if err := o.validateOrAskApp(); err != nil {
		return err
	}
	if err := o.validateAndAskSvcEnvName(); err != nil {
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

func (o *svcPauseOpts) validateOrAskApp() error {
	if o.appName != "" {
		_, err := o.store.GetApplication(o.appName)
		return err
	}
	app, err := o.sel.Application(svcPauseAppNamePrompt, wkldAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcPauseOpts) validateAndAskSvcEnvName() error {
	if o.envName != "" {
		if _, err := o.getTargetEnv(); err != nil {
			return err
		}
	}

	if o.svcName != "" {
		if _, err := o.store.GetService(o.appName, o.svcName); err != nil {
			return err
		}
	}

	// Note: we let prompter handle the case when there is only option for user to choose from.
	// This is naturally the case when `o.envName != "" && o.svcName != ""`.
	deployedService, err := o.sel.DeployedService(
		fmt.Sprintf(svcPauseNamePrompt, color.HighlightUserInput(o.appName)),
		svcPauseSvcNameHelpPrompt,
		o.appName,
		selector.WithEnv(o.envName),
		selector.WithName(o.svcName),
		selector.WithServiceTypesFilter([]string{manifestinfo.RequestDrivenWebServiceType}),
	)
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.appName, err)
	}
	o.svcName = deployedService.Name
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

func (o *svcPauseOpts) getTargetEnv() (*config.Environment, error) {
	if o.targetEnv != nil {
		return o.targetEnv, nil
	}
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	o.targetEnv = env
	return o.targetEnv, nil
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
