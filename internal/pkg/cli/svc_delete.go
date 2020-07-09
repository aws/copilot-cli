// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	svcDeleteNamePrompt       = "Which service would you like to delete?"
	fmtSvcDeleteConfirmPrompt = "Are you sure you want to delete %s from application %s?"
	svcDeleteConfirmHelp      = "This will remove the service from all environments, delete the local workspace file, and remove ECR repositories."
)

const (
	fmtSvcDeleteStart             = "Deleting service %s from environment %s."
	fmtSvcDeleteFailed            = "Failed to delete service %s from environment %s: %v."
	fmtSvcDeleteComplete          = "Deleted service %s from environment %s."
	fmtSvcDeleteResourcesStart    = "Deleting service %s resources from application %s."
	fmtSvcDeleteResourcesComplete = "Deleted service %s resources from application %s."
)

var (
	errSvcDeleteCancelled = errors.New("svc delete cancelled - no changes made")
)

type deleteSvcVars struct {
	*GlobalOpts
	SkipConfirmation bool
	Name             string
	EnvName          string
}

type deleteSvcOpts struct {
	deleteSvcVars

	// Interfaces to dependencies.
	store     store
	sess      sessionProvider
	spinner   progress
	appCFN    svcRemoverFromApp
	getSvcCFN func(session *awssession.Session) svcDeleter
	getECR    func(session *awssession.Session) imageRemover

	// Internal state.
	environments []*config.Environment
}

func newDeleteSvcOpts(vars deleteSvcVars) (*deleteSvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	provider := session.NewProvider()
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, err
	}

	return &deleteSvcOpts{
		deleteSvcVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(),
		sess:    provider,
		appCFN:  cloudformation.New(defaultSession),
		getSvcCFN: func(session *awssession.Session) svcDeleter {
			return cloudformation.New(session)
		},
		getECR: func(session *awssession.Session) imageRemover {
			return ecr.New(session)
		},
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deleteSvcOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.Name != "" {
		if _, err := o.store.GetService(o.AppName(), o.Name); err != nil {
			return err
		}
	}
	if o.EnvName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required flags.
func (o *deleteSvcOpts) Ask() error {
	if err := o.askSvcName(); err != nil {
		return err
	}

	if o.SkipConfirmation {
		return nil
	}

	deleteConfirmed, err := o.prompt.Confirm(
		fmt.Sprintf(fmtSvcDeleteConfirmPrompt, o.Name, o.appName),
		svcDeleteConfirmHelp)
	if err != nil {
		return fmt.Errorf("svc delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errSvcDeleteCancelled
	}
	return nil
}

// Execute deletes the application's CloudFormation stack, ECR repository, SSM parameter, and local file.
func (o *deleteSvcOpts) Execute() error {
	if err := o.appEnvironments(); err != nil {
		return err
	}

	if err := o.deleteStacks(); err != nil {
		return err
	}
	if err := o.emptyECRRepos(); err != nil {
		return err
	}
	if err := o.removeSvcFromApp(); err != nil {
		return err
	}
	if err := o.deleteSSMParam(); err != nil {
		return err
	}

	log.Successf("Deleted service %s from application %s.\n", o.Name, o.appName)
	return nil
}

func (o *deleteSvcOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deleteSvcOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.AppName(), o.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from config store: %w", o.EnvName, err)
	}
	return env, nil
}

func (o *deleteSvcOpts) askSvcName() error {
	if o.Name != "" {
		return nil
	}

	names, err := o.serviceNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("couldn't find any services in the application %s", o.AppName())
	}
	if len(names) == 1 {
		o.Name = names[0]
		log.Infof("Only found one service, defaulting to: %s\n", color.HighlightUserInput(o.Name))
		return nil
	}
	name, err := o.prompt.SelectOne(svcDeleteNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("select service to delete: %w", err)
	}
	o.Name = name
	return nil
}

func (o *deleteSvcOpts) serviceNames() ([]string, error) {
	services, err := o.store.ListServices(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list services for application %s: %w", o.AppName(), err)
	}
	var names []string
	for _, svc := range services {
		names = append(names, svc.Name)
	}
	return names, nil
}

func (o *deleteSvcOpts) appEnvironments() error {
	if o.EnvName != "" {
		env, err := o.targetEnv()
		if err != nil {
			return err
		}
		o.environments = append(o.environments, env)
	} else {
		envs, err := o.store.ListEnvironments(o.AppName())
		if err != nil {
			return fmt.Errorf("list environments: %w", err)
		}
		o.environments = envs
	}
	return nil
}

func (o *deleteSvcOpts) deleteStacks() error {
	for _, env := range o.environments {
		sess, err := o.sess.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}

		cfClient := o.getSvcCFN(sess)
		o.spinner.Start(fmt.Sprintf(fmtSvcDeleteStart, o.Name, env.Name))
		if err := cfClient.DeleteService(deploy.DeleteServiceInput{
			Name:    o.Name,
			EnvName: env.Name,
			AppName: o.appName,
		}); err != nil {
			o.spinner.Stop(log.Serrorf(fmtSvcDeleteFailed, o.Name, env.Name, err))
			return err
		}
		o.spinner.Stop(log.Ssuccessf(fmtSvcDeleteComplete, o.Name, env.Name))
	}
	return nil
}

// This is to make mocking easier in unit tests
func (o *deleteSvcOpts) emptyECRRepos() error {
	var uniqueRegions []string
	for _, env := range o.environments {
		if !contains(env.Region, uniqueRegions) {
			uniqueRegions = append(uniqueRegions, env.Region)
		}
	}

	// TODO: centralized ECR repo name
	repoName := fmt.Sprintf("%s/%s", o.appName, o.Name)
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

	o.spinner.Start(fmt.Sprintf(fmtSvcDeleteResourcesStart, o.Name, o.appName))
	if err := o.appCFN.RemoveServiceFromApp(proj, o.Name); err != nil {
		if !isStackSetNotExistsErr(err) {
			o.spinner.Stop(log.Serrorf(fmtSvcDeleteResourcesStart, o.Name, o.appName))
			return err
		}
	}
	o.spinner.Stop(log.Ssuccessf(fmtSvcDeleteResourcesComplete, o.Name, o.appName))
	return nil
}

func (o *deleteSvcOpts) deleteSSMParam() error {
	if err := o.store.DeleteService(o.appName, o.Name); err != nil {
		return fmt.Errorf("delete service %s in application %s from config store: %w", o.Name, o.appName, err)
	}

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deleteSvcOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to update the corresponding pipeline if it exists.",
			color.HighlightCode(fmt.Sprintf("copilot pipeline update"))),
	}
}

// BuildSvcDeleteCmd builds the command to delete application(s).
func BuildSvcDeleteCmd() *cobra.Command {
	vars := deleteSvcVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a service from an application.",
		Example: `
  Delete the "test" service.
  /code $ copilot svc delete --name test

  Delete the "test" service without confirmation prompt.
  /code $ copilot svc delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteSvcOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}

			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}

	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.SkipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
