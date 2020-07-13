// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/route53"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	fmtAppInitStart    = "Creating the infrastructure to manage services under application %s."
	fmtAppInitComplete = "Created the infrastructure to manage services under application %s.\n"
	fmtAppInitFailed   = "Failed to create the infrastructure to manage services under application %s.\n"

	fmtAppInitNamePrompt    = "What would you like to %s your application?"
	fmtAppInitNewNamePrompt = `Ok, let's create a new application then.
  What would you like to %s your application?`
	appInitNameHelpPrompt = "Services in the same application share the same VPC and ECS Cluster and are discoverable via service discovery."
)

type initAppVars struct {
	AppName      string
	DomainName   string
	ResourceTags map[string]string
}

type initAppOpts struct {
	initAppVars

	identity identityService
	store    applicationStore
	route53  domainValidator
	ws       wsAppManager
	cfn      appDeployer
	prompt   prompter
	prog     progress
}

func newInitAppOpts(vars initAppVars) (*initAppOpts, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	return &initAppOpts{
		initAppVars: vars,
		identity:    identity.New(sess),
		store:       store,
		route53:     route53.New(sess),
		ws:          ws,
		cfn:         cloudformation.New(sess),
		prompt:      prompt.New(),
		prog:        termprogress.NewSpinner(),
	}, nil
}

// Validate returns an error if the user's input is invalid.
func (o *initAppOpts) Validate() error {
	if o.AppName != "" {
		if err := o.validateAppName(o.AppName); err != nil {
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
func (o *initAppOpts) Ask() error {
	// When there's a local application.
	summary, err := o.ws.Summary()
	if err == nil {
		if o.AppName == "" {
			log.Infoln(fmt.Sprintf(
				"Your workspace is registered to application %s.",
				color.HighlightUserInput(summary.Application)))
			o.AppName = summary.Application
			return nil
		}
		if o.AppName != summary.Application {
			log.Errorf(`Workspace is already registered with application %s instead of %s.
If you'd like to delete the application locally, you can remove the %s directory.
If you'd like to delete the application and all of its resources, run %s.
`,
				summary.Application,
				o.AppName,
				workspace.CopilotDirName,
				color.HighlightCode("copilot app delete"))
			return fmt.Errorf("workspace already registered with %s", summary.Application)
		}
	}

	// Flag is set by user.
	if o.AppName != "" {
		return nil
	}

	existingApps, _ := o.store.ListApplications()
	if len(existingApps) == 0 {
		return o.askAppName(fmtAppInitNamePrompt)
	}

	useExistingApp, err := o.prompt.Confirm(
		"Would you like to use one of your existing applications?", "", prompt.WithTrueDefault(), prompt.WithFinalMessage("Use existing application:"))
	if err != nil {
		return fmt.Errorf("prompt to confirm using existing application: %w", err)
	}
	if useExistingApp {
		return o.askSelectExistingAppName(existingApps)
	}
	return o.askAppName(fmtAppInitNewNamePrompt)
}

// Execute creates a new managed empty application.
func (o *initAppOpts) Execute() error {
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	err = o.ws.Create(o.AppName)
	if err != nil {
		return fmt.Errorf("create new workspace with application name %s: %w", o.AppName, err)
	}
	o.prog.Start(fmt.Sprintf(fmtAppInitStart, color.HighlightUserInput(o.AppName)))
	err = o.cfn.DeployApp(&deploy.CreateAppInput{
		Name:           o.AppName,
		AccountID:      caller.Account,
		DomainName:     o.DomainName,
		AdditionalTags: o.ResourceTags,
	})
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtAppInitFailed, color.HighlightUserInput(o.AppName)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtAppInitComplete, color.HighlightUserInput(o.AppName)))

	return o.store.CreateApplication(&config.Application{
		AccountID: caller.Account,
		Name:      o.AppName,
		Domain:    o.DomainName,
		Tags:      o.ResourceTags,
	})
}

func (o *initAppOpts) validateAppName(name string) error {
	if err := validateAppName(name); err != nil {
		return err
	}
	app, err := o.store.GetApplication(name)
	if err != nil {
		var noSuchAppErr *config.ErrNoSuchApplication
		if errors.As(err, &noSuchAppErr) {
			return nil
		}
		return fmt.Errorf("get application %s: %w", name, err)
	}
	if app.Domain != o.DomainName {
		return fmt.Errorf("application named %s already exists with a different domain name %s", name, app.Domain)
	}
	return nil
}

func (o *initAppOpts) validateDomain(domainName string) error {
	domainExist, err := o.route53.DomainExists(domainName)
	if err != nil {
		return err
	}
	if !domainExist {
		return fmt.Errorf("no hosted zone found for %s", domainName)
	}
	return nil
}

// RecommendedActions returns a list of suggested additional commands users can run after successfully executing this command.
func (o *initAppOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to add a new service to your application.", color.HighlightCode("copilot init")),
	}
}

func (o *initAppOpts) askAppName(formatMsg string) error {
	appName, err := o.prompt.Get(
		fmt.Sprintf(formatMsg, color.Emphasize("name")),
		appInitNameHelpPrompt,
		validateAppName,
		prompt.WithFinalMessage("Application name:"))
	if err != nil {
		return fmt.Errorf("prompt get application name: %w", err)
	}
	o.AppName = appName
	return nil
}

func (o *initAppOpts) askSelectExistingAppName(existingApps []*config.Application) error {
	var names []string
	for _, p := range existingApps {
		names = append(names, p.Name)
	}
	name, err := o.prompt.SelectOne(
		fmt.Sprintf("Which %s do you want to add a new service to?", color.Emphasize("existing application")),
		appInitNameHelpPrompt,
		names,
		prompt.WithFinalMessage("Application name:"))
	if err != nil {
		return fmt.Errorf("prompt select application name: %w", err)
	}
	o.AppName = name
	return nil
}

// BuildAppInitCommand builds the command for creating a new application.
func BuildAppInitCommand() *cobra.Command {
	vars := initAppVars{}
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Creates a new empty application.",
		Long: `Creates a new empty application.
An application is a collection of containerized services that operate together.`,
		Example: `
  Create a new application named "test".
  /code $ copilot app init test
  Create a new application with an existing domain name in Amazon Route53.
  /code $ copilot app init --domain example.com
  Create a new application with resource tags.
  /code $ copilot app init --resource-tags department=MyDept,team=MyTeam`,
		Args: reservedArgs,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitAppOpts(vars)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				opts.AppName = args[0]
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
			log.Successf("The directory %s will hold service manifests for application %s.\n", color.HighlightResource(workspace.CopilotDirName), color.HighlightUserInput(opts.AppName))
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
