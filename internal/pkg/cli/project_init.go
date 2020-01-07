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

// Ask prompts the user for any required arguments that they didn't provide.
func (o *InitProjectOpts) Ask() error {
	// If there's a local project, we'll use that over anything else.
	summary, err := o.ws.Summary()
	if err == nil {
		msg := fmt.Sprintf(
			"Looks like you are using a workspace that's registered to project %s.\nWe'll use that as your project.",
			color.HighlightResource(summary.ProjectName))
		if o.ProjectName != "" && o.ProjectName != summary.ProjectName {
			msg = fmt.Sprintf(
				"Looks like you are using a workspace that's registered to project %s.\nWe'll use that as your project instead of %s.",
				color.HighlightResource(summary.ProjectName),
				color.HighlightUserInput(o.ProjectName))
		}
		log.Infoln(msg)
		o.ProjectName = summary.ProjectName
		return nil
	}

	if o.ProjectName != "" {
		// Flag is set by user.
		return nil
	}

	existingProjects, _ := o.projectStore.ListProjects()
	if len(existingProjects) == 0 {
		log.Infoln("Looks like you don't have any existing projects. Let's create one!")
		return o.askNewProjectName()
	}

	log.Infoln("Looks like you have some projects already.")
	useExistingProject, err := o.prompt.Confirm(
		"Would you like to use one of your existing projects?", "", prompt.WithTrueDefault())
	if err != nil {
		return fmt.Errorf("prompt to confirm using existing project: %w", err)
	}
	if useExistingProject {
		log.Infoln("Ok, here are your existing projects.")
		return o.askSelectExistingProjectName(existingProjects)
	}
	log.Infoln("Ok, let's create a new project then.")
	return o.askNewProjectName()
}

// Validate returns an error if the user's input is invalid.
func (o *InitProjectOpts) Validate() error {
	return validateProjectName(o.ProjectName)
}

// Execute creates a new managed empty project.
func (o *InitProjectOpts) Execute() error {
	caller, err := o.identity.Get()
	if err != nil {
		return err
	}

	err = o.projectStore.CreateProject(&archer.Project{
		AccountID: caller.Account,
		Name:      o.ProjectName,
		Domain:    o.DomainName,
	})
	if err != nil {
		// If the project already exists, move on - otherwise return the error.
		var projectAlreadyExistsError *store.ErrProjectAlreadyExists
		if !errors.As(err, &projectAlreadyExistsError) {
			return err
		}
	}
	err = o.ws.Create(o.ProjectName)
	if err != nil {
		return err
	}
	o.prog.Start(fmt.Sprintf(fmtDeployProjectStart, color.HighlightUserInput(o.ProjectName)))
	err = o.deployer.DeployProject(&deploy.CreateProjectInput{
		Project:    o.ProjectName,
		AccountID:  caller.Account,
		DomainName: o.DomainName,
	})
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtDeployProjectFailed, color.HighlightUserInput(o.ProjectName)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDeployProjectComplete, color.HighlightUserInput(o.ProjectName)))
	return nil
}

// RecommendedActions returns a list of suggested additional commands users can run after successfully executing this command.
func (o *InitProjectOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to add a new application to your project.", color.HighlightCode("ecs-preview init")),
	}
}

func (o *InitProjectOpts) askNewProjectName() error {
	projectName, err := o.prompt.Get(
		fmt.Sprintf("What would you like to %s your project?", color.Emphasize("name")),
		"Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
		validateProjectName)
	if err != nil {
		return fmt.Errorf("prompt get project name: %w", err)
	}
	o.ProjectName = projectName
	return nil
}

func (o *InitProjectOpts) askSelectExistingProjectName(existingProjects []*archer.Project) error {
	var projectNames []string
	for _, p := range existingProjects {
		projectNames = append(projectNames, p.Name)
	}
	projectName, err := o.prompt.SelectOne(
		fmt.Sprintf("Which %s do you want to add a new application to?", color.Emphasize("existing project")),
		"Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery.",
		projectNames)
	if err != nil {
		return fmt.Errorf("prompt select project name: %w", err)
	}
	o.ProjectName = projectName
	return nil
}

// BuildProjectInitCommand builds the command for creating a new project.
func BuildProjectInitCommand() *cobra.Command {
	opts := &InitProjectOpts{}

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Creates a new empty project.",
		Long: `Creates a new empty project.
A project is a collection of containerized applications (or micro-services) that operate together.`,
		Example: `
  Create a new project named test
  /code $ ecs-preview project init test`,
		Args: reservedArgs,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			sess, err := session.NewProvider().Default()
			if err != nil {
				return err
			}
			store, err := store.New()
			if err != nil {
				return err
			}
			ws, err := workspace.New()
			if err != nil {
				return err
			}

			opts.ws = ws
			opts.projectStore = store
			opts.identity = identity.New(sess)
			opts.deployer = cloudformation.New(sess)
			opts.prompt = prompt.New()
			opts.prog = termprogress.NewSpinner()
			return nil
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
