// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"

	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	awss3 "github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/cli/clean"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/s3"

	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/spf13/cobra"
)

const (
	svcDeleteNamePrompt              = "Which service would you like to delete?"
	fmtSvcDeleteConfirmPrompt        = "Are you sure you want to delete %s from application %s?"
	fmtSvcDeleteFromEnvConfirmPrompt = "Are you sure you want to delete %s from environment %s?"
	svcDeleteConfirmHelp             = "This will remove the service from all environments and delete it from your app."
	svcDeleteFromEnvConfirmHelp      = "This will remove the service from just the %s environment."
)

var (
	errSvcDeleteCancelled = errors.New("svc delete cancelled - no changes made")
)

type cleaner interface {
	Clean() error
}

type deleteSvcVars struct {
	appName          string
	skipConfirmation bool
	name             string
	envName          string
}

type deleteSvcOpts struct {
	deleteSvcVars

	// Interfaces to dependencies.
	store         store
	sess          sessionProvider
	spinner       progress
	prompt        prompter
	sel           configSelector
	appCFN        svcRemoverFromApp
	getSvcCFN     func(sess *awssession.Session) wlDeleter
	getECR        func(sess *awssession.Session) imageRemover
	newSvcCleaner func(sess *awssession.Session, env *config.Environment, manifestType string) cleaner
}

func newDeleteSvcOpts(vars deleteSvcVars) (*deleteSvcOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc delete"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	opts := &deleteSvcOpts{
		deleteSvcVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:  prompter,
		sess:    sessProvider,
		sel:     selector.NewConfigSelector(prompter, store),
		appCFN:  cloudformation.New(defaultSession, cloudformation.WithProgressTracker(os.Stderr)),
		getSvcCFN: func(sess *awssession.Session) wlDeleter {
			return cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr))
		},
		getECR: func(sess *awssession.Session) imageRemover {
			return ecr.New(sess)
		},
	}
	opts.newSvcCleaner = func(sess *awssession.Session, env *config.Environment, manifestType string) cleaner {
		if manifestType == manifestinfo.StaticSiteType {
			return clean.StaticSite(opts.appName, env.Name, opts.name, s3.New(sess), awss3.New(sess))
		}
		return &clean.NoOp{}
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *deleteSvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *deleteSvcOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
	} else {
		if err := o.askAppName(); err != nil {
			return err
		}
	}

	if o.name != "" {
		if _, err := o.store.GetService(o.appName, o.name); err != nil {
			return err
		}
	} else {
		if err := o.askSvcName(); err != nil {
			return err
		}
	}

	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	if o.skipConfirmation {
		return nil
	}

	// When there's no env name passed in, we'll completely
	// remove the service from the application.
	deletePrompt := fmt.Sprintf(fmtSvcDeleteConfirmPrompt, o.name, o.appName)
	deleteConfirmHelp := svcDeleteConfirmHelp
	if o.envName != "" {
		// When a customer provides a particular environment,
		// we'll just delete the service from that environment -
		// but keep it in the app.
		deletePrompt = fmt.Sprintf(fmtSvcDeleteFromEnvConfirmPrompt, o.name, o.envName)
		deleteConfirmHelp = fmt.Sprintf(svcDeleteFromEnvConfirmHelp, o.envName)
	}

	deleteConfirmed, err := o.prompt.Confirm(
		deletePrompt,
		deleteConfirmHelp,
		prompt.WithConfirmFinalMessage())

	if err != nil {
		return fmt.Errorf("svc delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errSvcDeleteCancelled
	}
	return nil
}

// Execute deletes the service's CloudFormation stack.
// If the service is being removed from the application, Execute will
// also delete the ECR repository and the SSM parameter.
func (o *deleteSvcOpts) Execute() error {
	wkld, err := o.store.GetWorkload(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get workload: %w", err)
	}

	envs, err := o.appEnvironments()
	if err != nil {
		return err
	}

	if err := o.deleteStacks(wkld.Type, envs); err != nil {
		return err
	}

	// Skip removing the service from the application if
	// we are only removing the stack from a particular environment.
	if !o.needsAppCleanup() {
		return nil
	}

	if err := o.emptyECRRepos(envs); err != nil {
		return err
	}
	if err := o.removeSvcFromApp(); err != nil {
		return err
	}
	if err := o.deleteSSMParam(); err != nil {
		return err
	}

	log.Infoln()
	log.Successf("Deleted service %s from application %s.\n", o.name, o.appName)

	return nil
}

func (o *deleteSvcOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deleteSvcOpts) needsAppCleanup() bool {
	// Only remove from a service from the app if
	// we're removing it from every environment.
	// If we're just removing the service from one
	// env, we keep the app configuration.
	return o.envName == ""
}

func (o *deleteSvcOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from config store: %w", o.envName, err)
	}
	return env, nil
}

func (o *deleteSvcOpts) askAppName() error {
	name, err := o.sel.Application(svcAppNamePrompt, wkldAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

func (o *deleteSvcOpts) askSvcName() error {
	name, err := o.sel.Service(svcDeleteNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *deleteSvcOpts) appEnvironments() ([]*config.Environment, error) {
	var envs []*config.Environment
	var err error
	if o.envName != "" {
		env, err := o.targetEnv()
		if err != nil {
			return nil, err
		}
		envs = append(envs, env)
	} else {
		envs, err = o.store.ListEnvironments(o.appName)
		if err != nil {
			return nil, fmt.Errorf("list environments: %w", err)
		}
	}
	return envs, nil
}

func (o *deleteSvcOpts) deleteStacks(wkldType string, envs []*config.Environment) error {
	for _, env := range envs {
		sess, err := o.sess.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}

		if err := o.newSvcCleaner(sess, env, wkldType).Clean(); err != nil {
			return fmt.Errorf("clean resources: %w", err)
		}

		cfClient := o.getSvcCFN(sess)
		if err := cfClient.DeleteWorkload(deploy.DeleteWorkloadInput{
			Name:             o.name,
			EnvName:          env.Name,
			AppName:          o.appName,
			ExecutionRoleARN: env.ExecutionRoleARN,
		}); err != nil {
			return fmt.Errorf("delete service: %w", err)
		}
	}
	return nil
}

// This is to make mocking easier in unit tests
func (o *deleteSvcOpts) emptyECRRepos(envs []*config.Environment) error {
	var uniqueRegions []string
	for _, env := range envs {
		if !slices.Contains(uniqueRegions, env.Region) {
			uniqueRegions = append(uniqueRegions, env.Region)
		}
	}

	// TODO: centralized ECR repo name
	repoName := clideploy.RepoName(o.appName, o.name)
	for _, region := range uniqueRegions {
		sess, err := o.sess.DefaultWithRegion(region)
		if err != nil {
			return err
		}
		client := o.getECR(sess)
		if err := client.ClearRepository(repoName); err != nil {
			return err
		}
	}
	return nil
}

func (o *deleteSvcOpts) removeSvcFromApp() error {
	proj, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}

	if err := o.appCFN.RemoveServiceFromApp(proj, o.name); err != nil {
		if !isStackSetNotExistsErr(err) {
			return err
		}
	}
	return nil
}

func (o *deleteSvcOpts) deleteSSMParam() error {
	if err := o.store.DeleteService(o.appName, o.name); err != nil {
		return fmt.Errorf("delete service %s in application %s from config store: %w", o.name, o.appName, err)
	}

	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deleteSvcOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Run %s to update the corresponding pipeline if it exists.",
			color.HighlightCode("copilot pipeline deploy")),
	})
	return nil
}

// buildSvcDeleteCmd builds the command to delete application(s).
func buildSvcDeleteCmd() *cobra.Command {
	vars := deleteSvcVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a service from an application.",
		Example: `
  Delete the "test" service from the application.
  /code $ copilot svc delete --name test

  Delete the "test" service from just the prod environment.
  /code $ copilot svc delete --name test --env prod

  Delete the "test" service from the "my-app" application from outside of the workspace.
  /code $ copilot svc delete --name test --app my-app

  Delete the "test" service without confirmation prompt.
  /code $ copilot svc delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteSvcOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}

	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
