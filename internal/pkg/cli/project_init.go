// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/route53"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
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

type initProjectVars struct {
	ProjectName  string
	DomainName   string
	ResourceTags map[string]string
}

type initProjectOpts struct {
	initProjectVars

	identity   identityService
	store      applicationStore
	route53Svc domainValidator
	ws         wsProjectManager
	deployer   projectDeployer
	prompt     prompter
	prog       progress
}

func newInitProjectOpts(vars initProjectVars) (*initProjectOpts, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, err
	}
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	return &initProjectOpts{
		initProjectVars: vars,
		identity:        identity.New(sess),
		store:           store,
		// See https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/DNSLimitations.html#limits-service-quotas
		// > To view limits and request higher limits for Route 53, you must change the Region to US East (N. Virginia).
		// So we have to set the region to us-east-1 to be able to find out if a domain name exists in the account.
		route53Svc: route53.New(sess),
		ws:         ws,
		deployer:   cloudformation.New(sess),
		prompt:     prompt.New(),
		prog:       termprogress.NewSpinner(),
	}, nil
}

// Validate returns an error if the user's input is invalid.
func (o *initProjectOpts) Validate() error {
	if o.ProjectName != "" {
		if err := o.validateProject(o.ProjectName); err != nil {
			return err
		}
	}
	if o.DomainName != "" {
		if err := o.validateDomain(o.DomainName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required arguments that they didn't provide.
func (o *initProjectOpts) Ask() error {
	// When there's a local project
	summary, err := o.ws.Summary()
	if err == nil {
		if o.ProjectName == "" {
			log.Infoln(fmt.Sprintf(
				"Looks like you are using a workspace that's registered to project %s.\nWe'll use that as your project.",
				color.HighlightResource(summary.Application)))
			o.ProjectName = summary.Application
			return nil
		}
		if o.ProjectName != summary.Application {
			log.Errorf(`Workspace is already registered with project %s instead of %s.
			If you'd like to delete the project locally, you can remove the %s directory.
			If you'd like to delete project and all of its resources, run %s.
`,
				summary.Application,
				o.ProjectName,
				workspace.CopilotDirName,
				color.HighlightCode("ecs-preview project delete"))
			return fmt.Errorf("workspace already registered with %s", summary.Application)
		}
	}

	// Flag is set by user.
	if o.ProjectName != "" {
		return nil
	}

	existingProjects, _ := o.store.ListApplications()
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

// Execute creates a new managed empty project.
func (o *initProjectOpts) Execute() error {
	caller, err := o.identity.Get()
	if err != nil {
		return err
	}

	err = o.ws.Create(o.ProjectName)
	if err != nil {
		return err
	}
	o.prog.Start(fmt.Sprintf(fmtDeployProjectStart, color.HighlightUserInput(o.ProjectName)))
	err = o.deployer.DeployApp(&deploy.CreateAppInput{
		Name:           o.ProjectName,
		AccountID:      caller.Account,
		DomainName:     o.DomainName,
		AdditionalTags: o.ResourceTags,
	})
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtDeployProjectFailed, color.HighlightUserInput(o.ProjectName)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDeployProjectComplete, color.HighlightUserInput(o.ProjectName)))

	return o.store.CreateApplication(&config.Application{
		AccountID: caller.Account,
		Name:      o.ProjectName,
		Domain:    o.DomainName,
		Tags:      o.ResourceTags,
	})
}

func (o *initProjectOpts) validateProject(projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}
	proj, err := o.store.GetApplication(projectName)
	if err != nil {
		var noSuchProjectErr *config.ErrNoSuchApplication
		if errors.As(err, &noSuchProjectErr) {
			return nil
		}
		return fmt.Errorf("get project %s: %w", projectName, err)
	}
	if proj.Domain != o.DomainName {
		return fmt.Errorf("project named %s already exists with a different domain name %s", projectName, proj.Domain)
	}

	return nil
}

func (o *initProjectOpts) validateDomain(domainName string) error {
	domainExist, err := o.route53Svc.DomainExists(domainName)
	if err != nil {
		return err
	}
	if !domainExist {
		return fmt.Errorf("no hosted zone found for %s", domainName)
	}

	return nil
}

// RecommendedActions returns a list of suggested additional commands users can run after successfully executing this command.
func (o *initProjectOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to add a new application to your project.", color.HighlightCode("ecs-preview init")),
	}
}

func (o *initProjectOpts) askNewProjectName() error {
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

func (o *initProjectOpts) askSelectExistingProjectName(existingProjects []*config.Application) error {
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
	vars := initProjectVars{}
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Creates a new empty project.",
		Long: `Creates a new empty project.
A project is a collection of containerized applications (or micro-services) that operate together.`,
		Example: `
  Create a new project named test
  /code $ ecs-preview project init test
  Create a new project with an existing domain name in Amazon Route53
  /code $ ecs-preview project init --domain example.com
  Create a new project with resource tags
  /code $ ecs-preview project init --resource-tags department=MyDept,team=MyTeam`,
		Args: reservedArgs,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitProjectOpts(vars)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				opts.ProjectName = args[0]
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
			log.Successf("The directory %s will hold application manifests for project %s.\n", color.HighlightResource(workspace.CopilotDirName), color.HighlightUserInput(opts.ProjectName))
			log.Infoln()
			log.Infoln("Recommended follow-up actions:")
			for _, followUp := range opts.RecommendedActions() {
				log.Infof("- %s\n", followUp)
			}
			return nil
		}),
	}
	cmd.Flags().StringVar(&vars.DomainName, domainNameFlag, "", domainNameFlagDescription)
	cmd.Flags().StringToStringVar(&vars.ResourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	return cmd
}
