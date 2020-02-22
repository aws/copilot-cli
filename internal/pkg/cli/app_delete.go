// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	appDeleteNamePrompt    = "Which application would you like to delete?"
	appDeleteConfirmPrompt = "Are you sure you want to delete %s from project %s?"
	appDeleteConfirmHelp   = "This will undeploy the app from all environments, delete the local workspace file, and remove ECR repositories."
)

var (
	errAppDeleteCancelled = errors.New("app delete cancelled - no changes made")
)

type deleteAppVars struct {
	*GlobalOpts
	SkipConfirmation bool
	AppName          string
	EnvName          string
}

type deleteAppOpts struct {
	deleteAppVars

	// Interfaces to dependencies.
	projectService   projectService
	workspaceService wsAppDeleter
	sessProvider     sessionProvider
	spinner          progress

	// Internal state.
	projectEnvironments []*archer.Environment
}

func newDeleteAppOpts(vars deleteAppVars) (*deleteAppOpts, error) {
	workspaceService, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("intialize workspace service: %w", err)
	}

	projectService, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("create project service: %w", err)
	}

	return &deleteAppOpts{
		deleteAppVars: vars,

		workspaceService: workspaceService,
		projectService:   projectService,
		spinner:          termprogress.NewSpinner(),
		sessProvider:     session.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deleteAppOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if o.AppName != "" {
		if _, err := o.projectService.GetApplication(o.ProjectName(), o.AppName); err != nil {
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
func (o *deleteAppOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}

	if o.SkipConfirmation {
		return nil
	}

	deleteConfirmed, err := o.prompt.Confirm(
		fmt.Sprintf(appDeleteConfirmPrompt, o.AppName, o.projectName),
		appDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("app delete confirmation prompt: %w", err)
	}

	if !deleteConfirmed {
		return errAppDeleteCancelled
	}

	return nil
}

// Execute deletes the application's CloudFormation stack, ECR repository, SSM parameter, and local file.
func (o *deleteAppOpts) Execute() error {
	if err := o.sourceProjectEnvironments(); err != nil {
		return err
	}

	if err := o.deleteStacks(); err != nil {
		return err
	}
	if err := o.emptyECRRepos(); err != nil {
		return err
	}
	if err := o.removeAppProjectResources(); err != nil {
		return err
	}
	if err := o.deleteSSMParam(); err != nil {
		return err
	}
	if err := o.deleteWorkspaceFile(); err != nil {
		return err
	}

	log.Successf("Deleted app %s from project %s.\n", o.AppName, o.projectName)
	return nil
}

func (o *deleteAppOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deleteAppOpts) targetEnv() (*archer.Environment, error) {
	env, err := o.projectService.GetEnvironment(o.ProjectName(), o.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from metadata store: %w", o.EnvName, err)
	}
	return env, nil
}

func (o *deleteAppOpts) askAppName() error {
	if o.AppName != "" {
		return nil
	}

	names, err := o.retrieveAppNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("couldn't find any application in the project %s", o.ProjectName())
	}
	name, err := o.prompt.SelectOne(appDeleteNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("select application to delete: %w", err)
	}
	o.AppName = name
	return nil
}

func (o *deleteAppOpts) retrieveAppNames() ([]string, error) {
	apps, err := o.projectService.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("get app names: %w", err)
	}
	var names []string
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names, nil
}

func (o *deleteAppOpts) sourceProjectEnvironments() error {

	if o.EnvName != "" {
		env, err := o.targetEnv()
		if err != nil {
			return err
		}
		o.projectEnvironments = append(o.projectEnvironments, env)
	} else {
		envs, err := o.projectService.ListEnvironments(o.ProjectName())

		if err != nil {
			return fmt.Errorf("get environments: %w", err)
		}

		o.projectEnvironments = envs
	}

	return nil
}

func (o *deleteAppOpts) deleteStacks() error {
	for _, env := range o.projectEnvironments {
		sess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}

		cfClient := cloudformation.New(sess)

		stackName := fmt.Sprintf("%s-%s-%s", o.projectName, env.Name, o.AppName)

		o.spinner.Start(fmt.Sprintf("Deleting app %s from env %s.", o.AppName, env.Name))
		if err := cfClient.DeleteStackAndWait(stackName); err != nil {
			o.spinner.Stop(log.Serrorf("Deleting app %s from env %s.", o.AppName, env.Name))

			return err
		}
		o.spinner.Stop(log.Ssuccessf("Deleted app %s from env %s.", o.AppName, env.Name))
	}

	return nil
}

func (o *deleteAppOpts) emptyECRRepos() error {
	var uniqueRegions []string
	for _, env := range o.projectEnvironments {
		if !contains(env.Region, uniqueRegions) {
			uniqueRegions = append(uniqueRegions, env.Region)
		}
	}

	// TODO: centralized ECR repo name
	repoName := fmt.Sprintf("%s/%s", o.projectName, o.AppName)

	for _, region := range uniqueRegions {
		sess, err := o.sessProvider.DefaultWithRegion(region)
		if err != nil {
			return err
		}

		ecrService := ecr.New(sess)

		if err := ecrService.ClearRepository(repoName); err != nil {
			return err
		}
	}

	return nil
}

func (o *deleteAppOpts) removeAppProjectResources() error {
	proj, err := o.projectService.GetProject(o.projectName)
	if err != nil {
		return err
	}

	sess, err := o.sessProvider.Default()
	if err != nil {
		return err
	}

	// TODO: make this opts.toolsAccountCfClient...
	cfClient := cloudformation.New(sess)

	o.spinner.Start(fmt.Sprintf("Deleting app %s resources from project %s.", o.AppName, o.projectName))
	if err := cfClient.RemoveAppFromProject(proj, o.AppName); err != nil {
		if !isStackSetNotExistsErr(err) {
			o.spinner.Stop(log.Serrorf("Deleting app %s resources from project %s.", o.AppName, o.projectName))
			return err
		}
	}
	o.spinner.Stop(log.Ssuccessf("Deleted app %s resources from project %s.", o.AppName, o.projectName))

	return nil
}

func (o *deleteAppOpts) deleteSSMParam() error {
	if err := o.projectService.DeleteApplication(o.projectName, o.AppName); err != nil {
		return fmt.Errorf("delete app %s from project %s: %w", o.AppName, o.projectName, err)
	}

	return nil
}

func (o *deleteAppOpts) deleteWorkspaceFile() error {
	if err := o.workspaceService.DeleteApp(o.AppName); err != nil {
		return fmt.Errorf("delete application %s directory: %w", o.AppName, err)
	}

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deleteAppOpts) RecommendedActions() []string {
	// TODO: Add recommendation to do `pipeline delete` when it is available
	return []string{
		fmt.Sprintf("Run %s to update the corresponding pipeline if it exists.",
			color.HighlightCode(fmt.Sprintf("ecs-preview pipeline update"))),
	}
}

// BuildAppDeleteCmd builds the command to delete application(s).
func BuildAppDeleteCmd() *cobra.Command {
	vars := deleteAppVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes an application from your project.",
		Example: `
  Delete the "test" application.
  /code $ ecs-preview app delete --name test

  Delete the "test" application without prompting.
	/code $ ecs-preview app delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteAppOpts(vars)
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

	cmd.Flags().StringVarP(&vars.AppName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.SkipConfirmation, yesFlag, false, yesFlagDescription)

	return cmd
}
