// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/cmd/archer/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/spinner"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const defaultEnvironmentName = "test"

// InitOpts holds the fields to bootstrap a new application.
type InitOpts struct {
	ShouldDeploy bool // true means we should create a test environment and deploy the application in it. Defaults to false.

	// Sub-commands to execute.
	initProject actionCommand
	initApp     actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	projectName    *string
	appType        *string
	appName        *string
	dockerfilePath *string

	// Interfaces for dependencies.
	envStore    archer.EnvironmentStore
	envDeployer archer.EnvironmentDeployer
	identity    identityService
	prog        progress
	prompt      prompter

	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.
}

func NewInitOpts() (*InitOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	ssm, err := ssm.NewStore()
	if err != nil {
		return nil, err
	}
	sess, err := session.Default()
	if err != nil {
		return nil, err
	}
	prompt := prompt.New()
	spin := spinner.New()

	initProject := &InitProjectOpts{
		projectStore: ssm,
		ws:           ws,
		prompt:       prompt,
	}
	initApp := &InitAppOpts{
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
		manifestWriter: ws,
		prompt:         prompt,
	}
	return &InitOpts{
		initProject: initProject,
		initApp:     initApp,

		projectName:    &initProject.ProjectName,
		appType:        &initApp.AppType,
		appName:        &initApp.AppName,
		dockerfilePath: &initApp.DockerfilePath,

		// TODO remove these dependencies from InitOpts after https://github.com/aws/amazon-ecs-cli-v2/issues/109
		envStore:    ssm,
		envDeployer: cloudformation.New(sess),
		identity:    identity.New(sess),
		prompt:      prompt,
		prog:        spin,
	}, nil
}

func (opts *InitOpts) Run() error {
	log.Warningln("It's best to run this command in the root of your workspace.")
	log.Infoln(`Welcome the the ECS CLI! We're going to walk you through some questions to help you get set up
with a project on ECS. A project is a collection of containerized applications (or micro-services)
that operate together.` + "\n")

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
	return opts.deploy()
}

func (opts *InitOpts) loadProject() error {
	if err := opts.initProject.Ask(); err != nil {
		return fmt.Errorf("prompt for project init: %w", err)
	}
	return opts.initProject.Validate()
}

func (opts *InitOpts) loadApp() error {
	if obj, ok := opts.initApp.(*InitAppOpts); ok {
		// To deploy an application we need to specify its project.
		obj.projectName = *opts.projectName
	}

	if err := opts.initApp.Ask(); err != nil {
		return fmt.Errorf("prompt for app init: %w", err)
	}
	return opts.initApp.Validate()
}

// deploy prompts the user to deploy a test environment if the project doesn't already have one.
func (opts *InitOpts) deploy() error {
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
	return opts.deployEnv()
}

func (opts *InitOpts) askShouldDeploy() error {
	v, err := opts.prompt.Confirm("Would you like to deploy a staging environment?", "A \"test\" environment with your application deployed to it. This will allow you to test your application before placing it in production.")
	if err != nil {
		return fmt.Errorf("failed to confirm deployment: %w", err)
	}
	opts.ShouldDeploy = v
	return nil
}

func (opts *InitOpts) deployEnv() error {
	identity, err := opts.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	// TODO https://github.com/aws/amazon-ecs-cli-v2/issues/56
	deployEnvInput := &archer.DeployEnvironmentInput{
		Project:                  *opts.projectName,
		Name:                     defaultEnvironmentName,
		PublicLoadBalancer:       true, // TODO: configure this value based on user input or Application type needs?
		ToolsAccountPrincipalARN: identity.ARN,
	}

	opts.prog.Start("Preparing deployment...")
	if err := opts.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("")
			log.Infof("The environment %s already exists under project %s.\n", deployEnvInput.Name, *opts.projectName)
			return nil
		}
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	opts.prog.Start("Deploying env...")
	env, err := opts.envDeployer.WaitForEnvironmentCreation(deployEnvInput)
	if err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	if err := opts.envStore.CreateEnvironment(env); err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	return nil
}

// BuildInitCmd builds the command for bootstrapping an application.
func BuildInitCmd() *cobra.Command {
	opts, err := NewInitOpts()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.promptForShouldDeploy = !cmd.Flags().Changed("deploy")
			return opts.Run()
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if !opts.ShouldDeploy {
				log.Info("\nNo problem, you can deploy your application later:\n")
				log.Infof("- Run %s to create your staging environment.\n",
					color.HighlightCode(fmt.Sprintf("archer env init --name %s --project %s", defaultEnvironmentName, *opts.projectName)))
				for _, followup := range opts.initApp.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
		},
	}
	cmd.Flags().StringVarP(opts.projectName, "project", "p", "", "Name of the project.")
	cmd.Flags().StringVarP(opts.appName, "app", "a", "", "Name of the application.")
	cmd.Flags().StringVarP(opts.appType, "app-type", "t", "", "Type of application to create.")
	cmd.Flags().StringVarP(opts.dockerfilePath, "dockerfile", "d", "", "Path to the Dockerfile.")
	cmd.Flags().BoolVar(&opts.ShouldDeploy, "deploy", false, "Deploy your application to a \"test\" environment.")
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
