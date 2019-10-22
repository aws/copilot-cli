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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
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
	Project      string
	ShouldDeploy bool // true means we should create a test environment and deploy the application in it. Defaults to false.

	// Sub-commands to execute.
	initApp actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	appType        *string
	appName        *string
	dockerfilePath *string

	// Interfaces for dependencies.
	projStore   archer.ProjectStore
	envStore    archer.EnvironmentStore
	envDeployer archer.EnvironmentDeployer
	ws          archer.Workspace
	identity    identityService
	prog        progress
	prompt      prompter

	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.
	existingProjects      []string
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

	initApp := &InitAppOpts{
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
		manifestWriter: ws,
		prompt:         prompt,
	}

	return &InitOpts{
		initApp: initApp,

		appType:        &initApp.AppType,
		appName:        &initApp.AppName,
		dockerfilePath: &initApp.DockerfilePath,

		// TODO remove these dependencies from InitOpts after https://github.com/aws/amazon-ecs-cli-v2/issues/109
		projStore:   ssm,
		envStore:    ssm,
		envDeployer: cloudformation.New(sess),
		ws:          ws,
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
		color.HighlightUserInput(*opts.appType), color.HighlightUserInput(*opts.appName), color.HighlightUserInput(opts.Project))

	if err := opts.createProject(); err != nil {
		return err
	}
	if err := opts.ws.Create(opts.Project); err != nil {
		return fmt.Errorf("create workspace for project %s: %w", opts.Project, err)
	}
	if err := opts.initApp.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}
	return opts.deploy()
}

func (opts *InitOpts) loadProject() error {
	// If there's a local project, we'll use that over anything else.
	summary, _ := opts.ws.Summary()
	if summary != nil {
		msg := fmt.Sprintf("Looks like you are using a workspace that's registered to project %s. We'll use that as your project.", color.HighlightUserInput(summary.ProjectName))
		if opts.Project != "" {
			msg = fmt.Sprintf("Looks like you are using a workspace that's registered to project %s. We'll use that as your project instead of %s.", color.HighlightUserInput(summary.ProjectName), color.HighlightUserInput(opts.Project))
		}
		log.Infoln(msg)
		opts.Project = summary.ProjectName
		return validateProjectName(opts.Project)
	}

	if opts.Project != "" {
		// Flag is set by user.
		return validateProjectName(opts.Project)
	}

	existingProjects, _ := opts.projStore.ListProjects()
	var projectNames []string
	for _, p := range existingProjects {
		projectNames = append(projectNames, p.Name)
	}
	opts.existingProjects = projectNames

	if err := opts.askProjectName(); err != nil {
		return err
	}
	return validateProjectName(opts.Project)
}

func (opts *InitOpts) loadApp() error {
	if obj, ok := opts.initApp.(*InitAppOpts); ok {
		obj.projectName = opts.Project
	}

	if err := opts.initApp.Ask(); err != nil {
		return fmt.Errorf("prompt for app init: %w", err)
	}
	return opts.initApp.Validate()
}

func (opts *InitOpts) askProjectName() error {
	if len(opts.existingProjects) == 0 {
		log.Infoln("Looks like you don't have any existing projects. Let's create one!")
		return opts.askNewProjectName()
	}

	log.Infoln("Looks like you have some projects already.")
	useExistingProject, err := opts.prompt.Confirm("Would you like to create a new app in one of your existing projects?", "", prompt.WithTrueDefault())
	if err != nil {
		return fmt.Errorf("failed to get new project confirmation: %w", err)
	}
	if useExistingProject {
		log.Infoln("Ok, here are your existing projects.")
		return opts.askSelectExistingProjectName()
	}
	log.Infoln("Ok, let's create a new project then.")
	return opts.askNewProjectName()
}

func (opts *InitOpts) askSelectExistingProjectName() error {
	projectName, err := opts.prompt.SelectOne(
		"Which one do you want to add a new application to?",
		"Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery.",
		opts.existingProjects)
	if err != nil {
		return fmt.Errorf("failed to get project selection: %w", err)
	}
	opts.Project = projectName
	return nil
}

func (opts *InitOpts) askNewProjectName() error {
	projectName, err := opts.prompt.Get(
		"What would you like to call your project?",
		"Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
		validateProjectName)
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}
	opts.Project = projectName
	return nil
}

func (opts *InitOpts) createProject() error {
	err := opts.projStore.CreateProject(&archer.Project{
		Name: opts.Project,
	})
	// If the project already exists, that's ok - otherwise
	// return the error.
	var projectAlreadyExistsError *store.ErrProjectAlreadyExists
	if !errors.As(err, &projectAlreadyExistsError) {
		return err
	}
	return nil
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
		Project:                  opts.Project,
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
			log.Infof("The environment %s already exists under project %s.\n", deployEnvInput.Name, opts.Project)
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
					color.HighlightCode(fmt.Sprintf("archer env init --name %s --project %s", defaultEnvironmentName, opts.Project)))
				for _, followup := range opts.initApp.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Name of the project.")
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
