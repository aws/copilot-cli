// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/deploy/cloudformation"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/manifest"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store/ssm"
	spin "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term/spinner"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/workspace"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
)

const defaultEnvironmentName = "test"

// InitAppOpts holds the fields to bootstrap a new application.
type InitAppOpts struct {
	// User provided fields
	Project          string `survey:"project"` // namespace that this application belongs to.
	Name             string `survey:"name"`    // unique identifier to logically group AWS resources together.
	Type             string `survey:"Type"`    // type of application you're trying to build (LoadBalanced, Backend, etc.)
	SkipDeploy       bool   // whether to skip asking if we should deploy a test environment.
	existingProjects []string

	projStore archer.ProjectStore
	envStore  archer.EnvironmentStore
	deployer  archer.EnvironmentDeployer
	ws        archer.Workspace
	spinner   spinner

	prompt terminal.Stdio // interfaces to receive and output app configuration data to the terminal.
}

// Ask prompts the user for the value of any required fields that are not already provided.
func (opts *InitAppOpts) Ask() error {
	var qs []*survey.Question
	if opts.Project == "" {
		qs = append(qs, opts.projectQuestion())
	}
	if opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name: "name",
			Prompt: &survey.Input{
				Message: "What is your application's name?",
				Help:    "Collection of AWS services to achieve a business capability. Must be unique within a project.",
			},
			Validate: validateApplicationName,
		})
	}
	if opts.Type == "" {
		qs = append(qs, opts.manifestQuestion())
	}
	return survey.Ask(qs, opts, survey.WithStdio(opts.prompt.In, opts.prompt.Out, opts.prompt.Err))
}

func (opts InitAppOpts) manifestQuestion() *survey.Question {
	return &survey.Question{
		Prompt: &survey.Select{
			Message: "Which template would you like to use?",
			Help:    "Pre-defined infrastructure templates.",
			Options: []string{
				manifest.LoadBalancedWebApplication,
			},
			Default: manifest.LoadBalancedWebApplication,
		},
		Name: "Type",
	}
}

func (opts InitAppOpts) projectQuestion() *survey.Question {
	if len(opts.existingProjects) > 0 {
		return &survey.Question{
			Name: "project",
			Prompt: &survey.Select{
				Message: "Which project should we use?",
				Help:    "Choose a project to create a new application in. Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery",
				Options: opts.existingProjects,
			},
		}
	}

	return &survey.Question{
		Name: "project",
		Prompt: &survey.Input{
			Message: "What is your project's name?",
			Help:    "Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
		},
		Validate: validateProjectName,
	}

}

// Validate returns an error if a command line flag provided value is invalid
func (opts *InitAppOpts) Validate() error {
	if err := validateProjectName(opts.Project); err != nil && err != errValueEmpty {
		return fmt.Errorf("project name invalid: %v", err)
	}

	if err := validateApplicationName(opts.Name); err != nil && err != errValueEmpty {
		return fmt.Errorf("application name invalid: %v", err)
	}

	return nil
}

// Prepare loads contextual data such as any existing projects, the current workspace, etc
func (opts *InitAppOpts) Prepare() {
	// If there's a local project, we'll use that and just skip the project question.
	// Otherwise, we'll load a list of existing projects that the customer can select from.
	if opts.Project != "" {
		return
	}
	if summary, err := opts.ws.Summary(); err == nil {
		// use the project name from the workspace
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

// Execute creates a project and initializes the workspace.
func (opts *InitAppOpts) Execute() error {
	if err := validateProjectName(opts.Project); err != nil {
		return err
	}

	if err := opts.createProject(); err != nil {
		return err
	}

	if err := opts.ws.Create(opts.Project); err != nil {
		return err
	}

	if err := opts.createApp(); err != nil {
		return err
	}

	return opts.deployEnv()
}
func (opts *InitAppOpts) createApp() error {
	manifest, err := manifest.Create(opts.Name, opts.Type)
	if err != nil {
		return fmt.Errorf("failed to generate a manifest %w", err)
	}
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the manifest file %w", err)
	}
	return opts.ws.WriteManifest(manifestBytes, opts.Name)
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

// deployEnv prompts the user to deploy a test environment if the project doesn't already have one.
func (opts *InitAppOpts) deployEnv() error {

	if opts.SkipDeploy {
		return nil
	}

	existingEnvs, _ := opts.envStore.ListEnvironments(opts.Project)
	if len(existingEnvs) > 0 {
		return nil
	}

	deployEnv := false
	prompt := &survey.Confirm{
		Message: "Would you like to set up a test environment?",
		Help:    "You can deploy your app into your test environment.",
	}
	survey.AskOne(prompt, &deployEnv, survey.WithStdio(opts.prompt.In, opts.prompt.Out, opts.prompt.Err))
	if !deployEnv {
		return nil
	}

	// TODO: prompt the user for an environment name with default value "test"
	// https://github.com/aws/PRIVATE-amazon-ecs-archer/issues/56
	env := &archer.Environment{
		Project:            opts.Project,
		Name:               defaultEnvironmentName,
		PublicLoadBalancer: true, // TODO: configure this value based on user input or Application type needs?
	}

	opts.spinner.Start("Preparing deployment...")
	if err := opts.deployer.DeployEnvironment(env); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.spinner.Stop("Done!")
			fmt.Printf("The environment %s already exists under project %s.\n", env.Name, opts.Project)
			return nil
		}
		opts.spinner.Stop("Error!")
		return err
	}
	opts.spinner.Stop("Done!")
	opts.spinner.Start("Deploying env...")
	if err := opts.deployer.WaitForEnvironmentCreation(env); err != nil {
		opts.spinner.Stop("Error!")
		return err
	}
	if err := opts.envStore.CreateEnvironment(env); err != nil {
		opts.spinner.Stop("Error!")
		return err
	}
	opts.spinner.Stop("Done!")
	return nil
}

// BuildInitCmd builds the command for bootstrapping an application.
func BuildInitCmd() *cobra.Command {
	opts := InitAppOpts{
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

			sess, err := session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
			})

			if err != nil {
				return err
			}

			opts.deployer = cloudformation.New(sess)

			opts.spinner = spin.New()

			opts.Prepare()
			return opts.Ask()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Execute()
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Name of the project (required).")
	cmd.Flags().StringVarP(&opts.Name, "app", "a", "", "Name of the application (required).")
	cmd.Flags().StringVarP(&opts.Type, "type", "t", "", "Type of application to create.")
	cmd.Flags().BoolVar(&opts.SkipDeploy, "skip-deploy", false, "Skips asking if you want to do a deployment.")
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Getting Started ✨",
	}
	return cmd
}
