// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the ecs-preview subcommands.
package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultEnvironmentName = "test"

const (
	initShouldDeployPrompt     = "Would you like to deploy a test environment?"
	initShouldDeployHelpPrompt = "An environment with your application deployed to it. This will allow you to test your application before placing it in production."
)

// InitOpts holds the fields to bootstrap a new application.
type InitOpts struct {
	// Flags unique to "init" that's not provided by other sub-commands.
	ShouldDeploy          bool // true means we should create a test environment and deploy the application in it. Defaults to false.
	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.

	// Sub-commands to execute.
	initProject actionCommand
	initApp     actionCommand
	initEnv     actionCommand
	appDeployer appDeployer

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	projectName    *string
	appType        *string
	appName        *string
	dockerfilePath *string

	prompt prompter
}

// NewInitOpts initiates the fields to bootstrap a new application.
func NewInitOpts() (*InitOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	ssm, err := store.New()
	if err != nil {
		return nil, err
	}
	sess, err := session.Default()
	if err != nil {
		return nil, err
	}
	prompt := prompt.New()
	spin := termprogress.NewSpinner()
	id := identity.New(sess)
	deployer := cloudformation.New(sess)

	initProject := &InitProjectOpts{
		projectStore: ssm,
		ws:           ws,
		prompt:       prompt,
		identity:     id,
		deployer:     deployer,
		prog:         spin,
	}
	initApp := &InitAppOpts{
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
		manifestWriter: ws,
		appStore:       ssm,
		projGetter:     ssm,
		projDeployer:   deployer,
		prog:           spin,
		GlobalOpts:     NewGlobalOpts(),
	}
	initEnv := &InitEnvOpts{
		EnvName:       defaultEnvironmentName,
		EnvProfile:    "default",
		IsProduction:  false,
		envCreator:    ssm,
		projectGetter: ssm,
		envDeployer:   deployer,
		projDeployer:  deployer, // TODO #317
		prog:          spin,
		identity:      id,
		GlobalOpts:    NewGlobalOpts(),
	}

	deployApp := &appDeployOpts{
		env: defaultEnvironmentName,

		spinner:       spin,
		dockerService: docker.New(),

		GlobalOpts: NewGlobalOpts(),
	}

	return &InitOpts{
		initProject: initProject,
		initApp:     initApp,
		initEnv:     initEnv,
		appDeployer: deployApp,

		projectName:    &initProject.ProjectName,
		appType:        &initApp.AppType,
		appName:        &initApp.AppName,
		dockerfilePath: &initApp.DockerfilePath,

		prompt: prompt,
	}, nil
}

// Run executes "project init", "env init", "app init" and "app deploy".
func (opts *InitOpts) Run() error {
	log.Warningln("It's best to run this command in the root of your Git repository.")
	log.Infoln(`Welcome to the ECS CLI! We're going to walk you through some questions 
to help you get set up with a project on ECS. A project is a collection of 
containerized applications (or micro-services) that operate together.`)
	log.Infoln()

	if err := opts.loadProject(); err != nil {
		return err
	}
	if err := opts.loadApp(); err != nil {
		return err
	}

	log.Infof("Ok great, we'll set up a %s named %s in project %s.\n",
		color.HighlightUserInput(*opts.appType), color.HighlightUserInput(*opts.appName), color.HighlightUserInput(*opts.projectName))

	if err := opts.initProject.Execute(); err != nil {
		return fmt.Errorf("execute project init: %w", err)
	}
	if err := opts.initApp.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}

	if err := opts.deployEnv(); err != nil {
		return err
	}

	return opts.deployApp()
}

func (opts *InitOpts) loadProject() error {
	if err := opts.initProject.Ask(); err != nil {
		return fmt.Errorf("prompt for project init: %w", err)
	}
	if err := opts.initProject.Validate(); err != nil {
		return err
	}
	// Write the project name to viper so that sub-commands can retrieve its value.
	viper.Set(projectFlag, opts.projectName)
	return nil
}

func (opts *InitOpts) loadApp() error {
	if err := opts.initApp.Ask(); err != nil {
		return fmt.Errorf("prompt for app init: %w", err)
	}
	return opts.initApp.Validate()
}

// deployEnv prompts the user to deploy a test environment if the project doesn't already have one.
func (opts *InitOpts) deployEnv() error {
	if opts.promptForShouldDeploy {
		log.Infoln("All right, you're all set for local development.")
		if err := opts.askShouldDeploy(); err != nil {
			return err
		}
	}
	if !opts.ShouldDeploy {
		// User chose not to deploy the application, exit.
		return nil
	}
	return opts.initEnv.Execute()
}

func (opts *InitOpts) deployApp() error {
	if !opts.ShouldDeploy {
		return nil
	}
	if deployOpts, ok := opts.appDeployer.(*appDeployOpts); ok {
		// Set the application's name to the deploy sub-command.
		deployOpts.app = *opts.appName
	}

	if err := opts.appDeployer.init(); err != nil {
		return err
	}

	if err := opts.appDeployer.sourceInputs(); err != nil {
		return err
	}

	return opts.appDeployer.deployApp()
}

func (opts *InitOpts) askShouldDeploy() error {
	v, err := opts.prompt.Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt)
	if err != nil {
		return fmt.Errorf("failed to confirm deployment: %w", err)
	}
	opts.ShouldDeploy = v
	return nil
}

// BuildInitCmd builds the command for bootstrapping an application.
func BuildInitCmd() *cobra.Command {
	opts, err := NewInitOpts()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application.",
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if *opts.dockerfilePath == "" {
				_, err = listDockerfiles(&afero.Afero{Fs: afero.NewOsFs()}, ".")
			}
			return err
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts.promptForShouldDeploy = !cmd.Flags().Changed(deployFlag)
			return opts.Run()
		}),
		PostRun: func(cmd *cobra.Command, args []string) {
			if !opts.ShouldDeploy {
				log.Info("\nNo problem, you can deploy your application later:\n")
				log.Infof("- Run %s to create your staging environment.\n",
					color.HighlightCode(fmt.Sprintf("ecs-preview env init --name %s --profile default --project %s", defaultEnvironmentName, *opts.projectName)))
				for _, followup := range opts.initApp.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
		},
	}
	cmd.Flags().StringVarP(opts.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	cmd.Flags().StringVarP(opts.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(opts.appType, appTypeFlag, appTypeFlagShort, "", appTypeFlagDescription)
	cmd.Flags().StringVarP(opts.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().BoolVar(&opts.ShouldDeploy, deployFlag, false, deployTestFlagDescription)
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
