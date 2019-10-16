// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/cmd/archer/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/spinner"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const defaultEnvironmentName = "test"

// InitAppOpts holds the fields to bootstrap a new application.
type InitAppOpts struct {
	Project               string // namespace that this application belongs to.
	AppName               string // unique identifier for the application.
	AppType               string // type of application you're trying to build (LoadBalanced, Backend, etc.)
	ShouldDeploy          bool   // true means we should create a test environment and deploy the application in it. Defaults to false.
	promptForShouldDeploy bool   // true means that the user set the ShouldDeploy flag explicitly.

	projStore   archer.ProjectStore
	envStore    archer.EnvironmentStore
	envDeployer archer.EnvironmentDeployer

	ws               archer.Workspace
	existingProjects []string

	prog     progress
	prompter prompter
}

// Prepare loads contextual data such as any existing projects, the current workspace, etc.
func (opts *InitAppOpts) Prepare() {
	log.Warningln("It's best to run this command in the root of your workspace.")
	log.Infoln(`Welcome the the ECS CLI! We're going to walk you through some questions to help you get set up
with a project on ECS. A project is a collection of containerized applications (or micro-services) 
that operate together.` + "\n")

	// If there's a local project, we'll use that and just skip the project question.
	// Otherwise, we'll load a list of existing projects that the customer can select from.
	if opts.Project != "" {
		return
	}
	if summary, err := opts.ws.Summary(); err == nil {
		log.Infof("Looks like you are using a workspace that's registered to project %s. We'll use that as your project.\n", color.HighlightUserInput(summary.ProjectName))
		opts.Project = summary.ProjectName
		return
	}
	// load all existing project names
	existingProjects, _ := opts.projStore.ListProjects()
	var projectNames []string
	for _, p := range existingProjects {
		projectNames = append(projectNames, p.Name)
	}
	opts.existingProjects = projectNames
}

// Ask prompts the user for the value of any required fields that are not already provided.
func (opts *InitAppOpts) Ask() error {
	if opts.Project == "" {
		if err := opts.askProjectName(); err != nil {
			return err
		}
	}
	if opts.AppType == "" {
		if err := opts.askAppType(); err != nil {
			return err
		}
	}
	if opts.AppName == "" {
		if err := opts.askAppName(); err != nil {
			return err
		}
	}
	return nil
}

// Validate returns an error if a command line flag provided value is invalid
func (opts *InitAppOpts) Validate() error {
	if err := validateProjectName(opts.Project); err != nil {
		return fmt.Errorf("project name %s is invalid: %w", opts.Project, err)
	}

	if err := validateApplicationName(opts.AppName); err != nil {
		return fmt.Errorf("application name %s is invalid: %w", opts.AppName, err)
	}

	// TODO validate application type

	return nil
}

// Execute creates a project and initializes the workspace.
func (opts *InitAppOpts) Execute() error {
	log.Infof("Ok great, we'll set up a %s named %s in project %s.\n",
		color.HighlightUserInput(opts.AppType), color.HighlightUserInput(opts.AppName), color.HighlightUserInput(opts.Project))

	if err := opts.createProject(); err != nil {
		return err
	}
	if err := opts.ws.Create(opts.Project); err != nil {
		return err
	}
	if err := opts.createManifest(); err != nil {
		return err
	}
	return opts.deploy()
}

func (opts *InitAppOpts) askProjectName() error {
	if len(opts.existingProjects) == 0 {
		log.Infoln("Looks like you don't have any existing projects. Let's create one!")
		return opts.askNewProjectName()
	}

	log.Infoln("Looks like you have some projects already.")
	useExistingProject, err := opts.prompter.Confirm("Would you like to create a new app in one of your existing projects?", "", prompt.WithTrueDefault())
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

func (opts *InitAppOpts) askSelectExistingProjectName() error {
	projectName, err := opts.prompter.SelectOne(
		"Which one do you want to add a new application to?",
		"Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery.",
		opts.existingProjects)
	if err != nil {
		return fmt.Errorf("failed to get project selection: %w", err)
	}
	opts.Project = projectName
	return nil
}

func (opts *InitAppOpts) askNewProjectName() error {
	projectName, err := opts.prompter.Get(
		"What would you like to call your project?",
		"Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
		validateProjectName)
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}
	opts.Project = projectName
	return nil
}

func (opts *InitAppOpts) askAppType() error {
	t, err := opts.prompter.SelectOne(
		"What type of application do you want to make?",
		"List of infrastructure patterns.",
		manifest.AppTypes)

	if err != nil {
		return fmt.Errorf("failed to get type selection: %w", err)
	}
	opts.AppType = t
	return nil
}

func (opts *InitAppOpts) askAppName() error {
	name, err := opts.prompter.Get(
		fmt.Sprintf("What do you want to call this %s?", opts.AppType),
		"Collection of AWS services to achieve a business capability. Must be unique within a project.",
		validateApplicationName)
	if err != nil {
		return fmt.Errorf("failed to get application name: %w", err)
	}
	opts.AppName = name
	return nil
}

func (opts *InitAppOpts) askShouldDeploy() error {
	v, err := opts.prompter.Confirm("Would you like to deploy a staging environment?", "A \"test\" environment with your application deployed to it. This will allow you to test your application before placing it in production.")
	if err != nil {
		return fmt.Errorf("failed to confirm deployment: %w", err)
	}
	opts.ShouldDeploy = v
	return nil
}

func (opts *InitAppOpts) createProject() error {
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

func (opts *InitAppOpts) createManifest() error {
	manifest, err := manifest.CreateApp(opts.AppName, opts.AppType)
	if err != nil {
		return fmt.Errorf("failed to generate a manifest %w", err)
	}
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the manifest file %w", err)
	}
	manifestPath, err := opts.ws.WriteManifest(manifestBytes, opts.AppName)
	if err != nil {
		return fmt.Errorf("failed to write manifest for app %s: %w", opts.AppName, err)
	}

	wkdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	relPath, err := filepath.Rel(wkdir, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path of manifest file: %w", err)
	}
	log.Infoln()
	log.Successf("Wrote the manifest for %s app at '%s'\n", color.HighlightUserInput(opts.AppName), color.HighlightResource(relPath))
	log.Infoln()
	return nil
}

// deploy prompts the user to deploy a test environment if the project doesn't already have one.
func (opts *InitAppOpts) deploy() error {
	if opts.promptForShouldDeploy {
		log.Infoln("All right, you're all set for local development.")
		if err := opts.askShouldDeploy(); err != nil {
			return err
		}
		if !opts.ShouldDeploy {
			log.Infoln()
			log.Infoln("No problem, you can deploy your application later:")
			log.Infof("- Run %s to create your staging environment.\n",
				color.HighlightCode(fmt.Sprintf("archer env init --env %s --project %s", defaultEnvironmentName, opts.Project)))
			log.Infof("- Run %s to deploy your application to the environment.\n",
				color.HighlightCode(fmt.Sprintf("archer env add --app %s --env %s --project %s", opts.AppName, defaultEnvironmentName, opts.Project)))
			log.Infoln()
		}
	}
	if !opts.ShouldDeploy {
		// User chose not to deploy the application, exit.
		return nil
	}
	return opts.deployEnv()
}

func (opts *InitAppOpts) deployEnv() error {
	// TODO https://github.com/aws/amazon-ecs-cli-v2/issues/56
	env := &archer.Environment{
		Project:            opts.Project,
		Name:               defaultEnvironmentName,
		PublicLoadBalancer: true, // TODO: configure this value based on user input or Application type needs?
	}

	opts.prog.Start("Preparing deployment...")
	if err := opts.envDeployer.DeployEnvironment(env); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("")
			log.Infof("The environment %s already exists under project %s.\n", env.Name, opts.Project)
			return nil
		}
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	opts.prog.Start("Deploying env...")
	if err := opts.envDeployer.WaitForEnvironmentCreation(env); err != nil {
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
	opts := InitAppOpts{
		prompter: prompt.New(),
		prog:     spinner.New(),
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// setup dependencies
			ws, err := workspace.New()
			if err != nil {
				return err
			}
			opts.ws = ws

			ssm, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.projStore = ssm
			opts.envStore = ssm

			sess, err := session.Default()
			if err != nil {
				return err
			}
			opts.envDeployer = cloudformation.New(sess)
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.Prepare()
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.promptForShouldDeploy = !cmd.Flags().Changed("deploy")
			return opts.Execute()
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Name of the project.")
	cmd.Flags().StringVarP(&opts.AppName, "app", "a", "", "Name of the application.")
	cmd.Flags().StringVarP(&opts.AppType, "app-type", "t", "", "Type of application to create.")
	cmd.Flags().BoolVar(&opts.ShouldDeploy, "deploy", false, "Deploy your application to a \"test\" environment.")
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
