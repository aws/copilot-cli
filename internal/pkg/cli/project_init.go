// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
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
	fmtDeployProjectStart    = "Creating the infrastructure to manage container repositories under project %s."
	fmtDeployProjectComplete = "Created the infrastructure to manage container repositories under project %s."
	fmtDeployProjectFailed   = "Failed to create the infrastructure to manage container repositories under project %s."
)

// InitProjectOpts contains the fields to collect for creating a project.
type InitProjectOpts struct {
	ProjectName string
	DomainName  string

	identity     identityService
	projectStore archer.ProjectStore
	ws           archer.Workspace
	deployer     projectDeployer
	prompt       prompter
	prog         progress
}

// NewInitProjectOpts returns a new InitProjectOpts.
func NewInitProjectOpts() (*InitProjectOpts, error) {
	p := session.NewProvider()
	defaultSession, err := p.Default()
	if err != nil {
		return nil, err
	}

	store, err := store.New()
	if err != nil {
		return nil, err
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	return &InitProjectOpts{
		identity:     identity.New(defaultSession),
		projectStore: store,
		ws:           ws,
		deployer:     cloudformation.New(defaultSession),
		prompt:       prompt.New(),
		prog:         termprogress.NewSpinner(),
	}, nil
}

// Ask prompts the user for any required arguments that they didn't provide.
func (opts *InitProjectOpts) Ask() error {
	// If there's a local project, we'll use that over anything else.
	summary, err := opts.ws.Summary()
	if err == nil {
		msg := fmt.Sprintf(
			"Looks like you are using a workspace that's registered to project %s.\nWe'll use that as your project.",
			color.HighlightResource(summary.ProjectName))
		if opts.ProjectName != "" && opts.ProjectName != summary.ProjectName {
			msg = fmt.Sprintf(
				"Looks like you are using a workspace that's registered to project %s.\nWe'll use that as your project instead of %s.",
				color.HighlightResource(summary.ProjectName),
				color.HighlightUserInput(opts.ProjectName))
		}
		log.Infoln(msg)
		opts.ProjectName = summary.ProjectName
		return nil
	}

	if opts.ProjectName != "" {
		// Flag is set by user.
		return nil
	}

	existingProjects, _ := opts.projectStore.ListProjects()
	if len(existingProjects) == 0 {
		log.Infoln("Looks like you don't have any existing projects. Let's create one!")
		return opts.askNewProjectName()
	}

	log.Infoln("Looks like you have some projects already.")
	useExistingProject, err := opts.prompt.Confirm(
		"Would you like to use one of your existing projects?", "", prompt.WithTrueDefault())
	if err != nil {
		return fmt.Errorf("prompt to confirm using existing project: %w", err)
	}
	if useExistingProject {
		log.Infoln("Ok, here are your existing projects.")
		return opts.askSelectExistingProjectName(existingProjects)
	}
	log.Infoln("Ok, let's create a new project then.")
	return opts.askNewProjectName()
}

// Validate returns an error if the user's input is invalid.
func (opts *InitProjectOpts) Validate() error {
	return validateProjectName(opts.ProjectName)
}

// Execute creates a new managed empty project.
func (opts *InitProjectOpts) Execute() error {
	caller, err := opts.identity.Get()
	if err != nil {
		return err
	}

	err = opts.projectStore.CreateProject(&archer.Project{
		AccountID: caller.Account,
		Name:      opts.ProjectName,
		Domain:    opts.DomainName,
	})
	if err != nil {
		// If the project already exists, move on - otherwise return the error.
		var projectAlreadyExistsError *store.ErrProjectAlreadyExists
		if !errors.As(err, &projectAlreadyExistsError) {
			return err
		}
	}
	err = opts.ws.Create(opts.ProjectName)
	if err != nil {
		return err
	}
	opts.prog.Start(fmt.Sprintf(fmtDeployProjectStart, color.HighlightUserInput(opts.ProjectName)))
	err = opts.deployer.DeployProject(&deploy.CreateProjectInput{
		Project:    opts.ProjectName,
		AccountID:  caller.Account,
		DomainName: opts.DomainName,
	})
	if err != nil {
		opts.prog.Stop(log.Serrorf(fmtDeployProjectFailed, color.HighlightUserInput(opts.ProjectName)))
		return err
	}
	opts.prog.Stop(log.Ssuccessf(fmtDeployProjectComplete, color.HighlightUserInput(opts.ProjectName)))
	return nil
}

// RecommendedActions returns a list of suggested additional commands users can run after successfully executing this command.
func (opts *InitProjectOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to add a new application to your project.", color.HighlightCode("dw_run.sh init")),
	}
}

func (opts *InitProjectOpts) askNewProjectName() error {
	projectName, err := opts.prompt.Get(
		fmt.Sprintf("What would you like to %s your project?", color.Emphasize("name")),
		"Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
		validateProjectName)
	if err != nil {
		return fmt.Errorf("prompt get project name: %w", err)
	}
	opts.ProjectName = projectName
	return nil
}

func (opts *InitProjectOpts) askSelectExistingProjectName(existingProjects []*archer.Project) error {
	var projectNames []string
	for _, p := range existingProjects {
		projectNames = append(projectNames, p.Name)
	}
	projectName, err := opts.prompt.SelectOne(
		fmt.Sprintf("Which %s do you want to add a new application to?", color.Emphasize("existing project")),
		"Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery.",
		projectNames)
	if err != nil {
		return fmt.Errorf("prompt select project name: %w", err)
	}
	opts.ProjectName = projectName
	return nil
}

// BuildProjectInitCommand builds the command for creating a new project.
func BuildProjectInitCommand() *cobra.Command {
	opts, err := NewInitProjectOpts()

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Creates a new empty project.",
		Long: `Creates a new empty project.
A project is a collection of containerized applications (or micro-services) that operate together.`,
		Example: `
  Create a new project named test
  /code $ dw_run.sh project init test`,
		Args: reservedArgs,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return err
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.ProjectName = args[0]
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		}),
		PostRun: func(cmd *cobra.Command, args []string) {
			log.Successf("The directory %s will hold application manifests for project %s.\n", color.HighlightResource(workspace.ProjectDirectoryName), color.HighlightUserInput(opts.ProjectName))
			log.Infoln()
			log.Infoln("Recommended follow-up actions:")
			for _, followUp := range opts.RecommendedActions() {
				log.Infof("- %s\n", followUp)
			}
		},
	}
	cmd.Flags().StringVar(&opts.DomainName, domainNameFlag, "", domainNameFlagDescription)
	return cmd
}
