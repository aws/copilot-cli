// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	appDeleteConfirmPrompt = "Are you sure you want to delete %s from project %s?"
	appDeleteConfirmHelp   = "This will undeploy the app from all environments, delete the local workspace file, and remove ECR repositories."
)

var (
	errAppDeleteCancelled = errors.New("app delete cancelled - no changes made")
)

// BuildAppDeleteCmd builds the command to delete application(s).
func BuildAppDeleteCmd() *cobra.Command {
	opts := &deleteAppOpts{
		GlobalOpts: NewGlobalOpts(),
		spinner:    termprogress.NewSpinner(),
		prompter:   prompt.New(),
	}

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an application from your project.",
		Example: `
  Delete the "test" application.
  /code $ archer app delete test

  Delete the "test" application without prompting.
  /code $ archer app delete test --yes`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires a single application name as argument")
			}

			opts.app = args[0]

			if err := opts.init(); err != nil {
				return err
			}

			if err := opts.sourceInputs(); err != nil {
				return err
			}

			return opts.confirmDelete()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return opts.deleteApp()
		}),
	}

	cmd.Flags().BoolVar(&opts.skipConfirmation, yesFlag, false, yesFlagDescription)

	return cmd
}

type deleteAppOpts struct {
	*GlobalOpts
	app              string
	env              string
	skipConfirmation bool

	projectService   projectService
	workspaceService archer.Workspace
	spinner          progress
	prompter         prompter

	localProjectAppNames []string
	projectEnvironments  []*archer.Environment
}

func (opts deleteAppOpts) confirmDelete() error {
	if opts.skipConfirmation {
		return nil
	}

	deleteConfirmed, err := opts.prompter.Confirm(
		fmt.Sprintf(appDeleteConfirmPrompt, opts.app, opts.projectName),
		appDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("app delete confirmation prompt: %w", err)
	}

	if !deleteConfirmed {
		return errAppDeleteCancelled
	}

	return nil
}

func (opts *deleteAppOpts) init() error {
	projectService, err := store.New()
	if err != nil {
		return fmt.Errorf("create project service: %w", err)
	}
	opts.projectService = projectService

	workspaceService, err := workspace.New()
	if err != nil {
		return fmt.Errorf("intialize workspace service: %w", err)
	}
	opts.workspaceService = workspaceService

	return nil
}

func (opts *deleteAppOpts) sourceInputs() error {
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	if err := opts.sourceProjectData(); err != nil {
		return err
	}

	return nil
}

func (opts *deleteAppOpts) sourceProjectData() error {
	if err := opts.sourceWorkspaceApplications(); err != nil {
		return err
	}

	if err := opts.sourceProjectEnvironments(); err != nil {
		return err
	}

	return nil
}

func (opts *deleteAppOpts) sourceWorkspaceApplications() error {
	apps, err := opts.workspaceService.Apps()

	if err != nil {
		return fmt.Errorf("get app names: %w", err)
	}

	if len(apps) == 0 {
		// TODO: recommend follow up command - app init?
		return errors.New("no applications found")
	}

	var appNames []string
	for _, app := range apps {
		appNames = append(appNames, app.AppName())
	}

	opts.localProjectAppNames = appNames

	return nil
}

func (opts *deleteAppOpts) sourceProjectEnvironments() error {
	envs, err := opts.projectService.ListEnvironments(opts.ProjectName())

	if err != nil {
		return fmt.Errorf("get environments: %w", err)
	}

	if len(envs) == 0 {
		log.Infof("couldn't find any environments associated with project %s, try initializing one: %s\n",
			color.HighlightUserInput(opts.ProjectName()),
			color.HighlightCode("archer env init"))

		return errors.New("no environments found")
	}

	opts.projectEnvironments = envs

	return nil
}

func (opts deleteAppOpts) deleteApp() error {
	if err := opts.deleteStacks(); err != nil {
		return err
	}
	if err := opts.emptyECRRepos(); err != nil {
		return err
	}
	if err := opts.removeAppProjectResources(); err != nil {
		return err
	}
	if err := opts.deleteSSMParam(); err != nil {
		return err
	}
	if err := opts.deleteWorkspaceFile(); err != nil {
		return err
	}

	log.Ssuccessf("removed app %s from project %s\n", opts.app, opts.projectName)

	return nil
}

func (opts deleteAppOpts) deleteStacks() error {
	for _, env := range opts.projectEnvironments {
		sess, err := session.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}

		cfClient := cloudformation.New(sess)

		stackName := fmt.Sprintf("%s-%s-%s", opts.projectName, env.Name, opts.app)

		// TODO: check if the stack exists first
		opts.spinner.Start(fmt.Sprintf("deleting app %s from env %s", opts.app, env.Name))
		if err := cfClient.DeleteStackAndWait(stackName); err != nil {
			opts.spinner.Stop(log.Serrorf("deleting app %s from env %s", opts.app, env.Name))

			return err
		}
		opts.spinner.Stop(log.Ssuccessf("deleted app %s from env %s", opts.app, env.Name))
	}

	return nil
}

func (opts deleteAppOpts) emptyECRRepos() error {
	var regions []string
	for _, env := range opts.projectEnvironments {
		if !contains(env.Region, regions) {
			regions = append(regions, env.Region)
		}
	}

	// TODO: centralized ECR repo name
	repoName := fmt.Sprintf("%s/%s", opts.projectName, opts.app)

	for _, uniqueRegions := range regions {
		sess, err := session.DefaultWithRegion(uniqueRegions)
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

func (opts deleteAppOpts) removeAppProjectResources() error {
	proj, err := opts.projectService.GetProject(opts.projectName)
	if err != nil {
		return err
	}

	sess, err := session.Default()
	if err != nil {
		return err
	}

	// TODO: make this opts.toolsAccountCfClient...
	cfClient := cloudformation.New(sess)

	opts.spinner.Start(fmt.Sprintf("removing app %s resources from project %s", opts.app, opts.projectName))
	if err := cfClient.RemoveAppFromProject(proj, opts.app); err != nil {
		opts.spinner.Stop(log.Serrorf("removing app %s resources from project %s", opts.app, opts.projectName))

		return err
	}
	opts.spinner.Stop(log.Ssuccessf("removed app %s resources from project %s", opts.app, opts.projectName))

	return nil
}

func (opts deleteAppOpts) deleteSSMParam() error {
	return opts.projectService.DeleteApplication(opts.projectName, opts.app)
}

func (opts deleteAppOpts) deleteWorkspaceFile() error {
	// TODO: move to a workspace method? workspace.DeleteApp(name string)?
	filePath := filepath.Join("ecs-project", fmt.Sprintf("%s-app.yml", opts.app))

	if err := os.Remove(filePath); err != nil {
		return err
	}

	log.Infoln("deleted file:", filePath)

	return nil
}
