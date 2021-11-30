// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/route53"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	fmtAppInitStart    = "Creating the infrastructure to manage services and jobs under application %s."
	fmtAppInitComplete = "Created the infrastructure to manage services and jobs under application %s.\n\n"
	fmtAppInitFailed   = "Failed to create the infrastructure to manage services and jobs under application %s.\n\n"

	fmtAppInitNamePrompt    = "What would you like to %s your application?"
	fmtAppInitNewNamePrompt = `Ok, let's create a new application then.
  What would you like to %s your application?`
	appInitNameHelpPrompt = "Services and jobs in the same application share the same VPC and ECS Cluster and services are discoverable via service discovery."
)

type initAppVars struct {
	name         string
	domainName   string
	resourceTags map[string]string
}

type initAppOpts struct {
	initAppVars

	identity             identityService
	store                applicationStore
	route53              domainHostedZoneGetter
	domainInfoGetter     domainInfoGetter
	ws                   wsAppManager
	cfn                  appDeployer
	prompt               prompter
	prog                 progress
	isSessionFromEnvVars func() (bool, error)

	cachedHostedZoneID        string
	isDomainNotFoundInAccount bool
}

func newInitAppOpts(vars initAppVars) (*initAppOpts, error) {
	sess, err := sessions.NewProvider().Default()
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
		initAppVars:      vars,
		identity:         identity.New(sess),
		store:            store,
		route53:          route53.New(sess),
		domainInfoGetter: route53.NewRoute53Domains(sess),
		ws:               ws,
		cfn:              cloudformation.New(sess),
		prompt:           prompt.New(),
		prog:             termprogress.NewSpinner(log.DiagnosticWriter),
		isSessionFromEnvVars: func() (bool, error) {
			return sessions.AreCredsFromEnvVars(sess)
		},
	}, nil
}

// Validate returns an error if the user's input is invalid.
func (o *initAppOpts) Validate() error {
	if o.name != "" {
		if err := o.validateAppName(o.name); err != nil {
			return err
		}
	}
	if o.domainName != "" {
		if err := validateDomainName(o.domainName); err != nil {
			return fmt.Errorf("domain name %s is invalid: %w", o.domainName, err)
		}
		if err := o.isDomainOwned(); err != nil {
			return err
		}
		id, err := o.domainHostedZoneID(o.domainName)
		if err != nil {
			return err
		}
		o.cachedHostedZoneID = id
	}
	return nil
}

// Ask prompts the user for any required arguments that they didn't provide.
func (o *initAppOpts) Ask() error {
	ok, err := o.isSessionFromEnvVars()
	if err != nil {
		return err
	}

	if ok {
		log.Warningln(`Looks like you're creating an application using credentials set by environment variables.
Copilot will store your application metadata in this account.
We recommend using credentials from named profiles. To learn more:
https://aws.github.io/copilot-cli/docs/credentials/`)
		log.Infoln()
	}

	// When there's a local application.
	summary, err := o.ws.Summary()
	if err == nil {
		if o.name == "" {
			log.Infoln(fmt.Sprintf(
				"Your workspace is registered to application %s.",
				color.HighlightUserInput(summary.Application)))
			o.name = summary.Application
			return nil
		}
		if o.name != summary.Application {
			log.Errorf(`Workspace is already registered with application %s instead of %s.
If you'd like to delete the application locally, you can remove the %s directory.
If you'd like to delete the application and all of its resources, run %s.
`,
				summary.Application,
				o.name,
				workspace.CopilotDirName,
				color.HighlightCode("copilot app delete"))
			return fmt.Errorf("workspace already registered with %s", summary.Application)
		}
	}

	// Flag is set by user.
	if o.name != "" {
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

	err = o.ws.Create(o.name)
	if err != nil {
		return fmt.Errorf("create new workspace with application name %s: %w", o.name, err)
	}
	o.prog.Start(fmt.Sprintf(fmtAppInitStart, color.HighlightUserInput(o.name)))
	var hostedZoneID string
	if o.domainName != "" {
		hostedZoneID, err = o.domainHostedZoneID(o.domainName)
		if err != nil {
			return err
		}
	}
	err = o.cfn.DeployApp(&deploy.CreateAppInput{
		Name:               o.name,
		AccountID:          caller.Account,
		DomainName:         o.domainName,
		DomainHostedZoneID: hostedZoneID,
		AdditionalTags:     o.resourceTags,
		Version:            deploy.LatestAppTemplateVersion,
	})
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtAppInitFailed, color.HighlightUserInput(o.name)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtAppInitComplete, color.HighlightUserInput(o.name)))

	if err := o.store.CreateApplication(&config.Application{
		AccountID:          caller.Account,
		Name:               o.name,
		Domain:             o.domainName,
		DomainHostedZoneID: hostedZoneID,
		Tags:               o.resourceTags,
	}); err != nil {
		return err
	}
	log.Successf("The directory %s will hold service manifests for application %s.\n", color.HighlightResource(workspace.CopilotDirName), color.HighlightUserInput(o.name))
	log.Infoln()
	return nil
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
	if o.domainName != "" && app.Domain != o.domainName {
		return fmt.Errorf("application named %s already exists with a different domain name %s", name, app.Domain)
	}
	return nil
}

func (o *initAppOpts) isDomainOwned() error {
	err := o.domainInfoGetter.IsDomainOwned(o.domainName)
	if err == nil {
		return nil
	}
	var errDomainNotFound *route53.ErrDomainNotFound
	if errors.As(err, &errDomainNotFound) {
		o.isDomainNotFoundInAccount = true
		return nil
	}
	return fmt.Errorf("check if domain is owned by the account: %w", err)
}

func (o *initAppOpts) domainHostedZoneID(domainName string) (string, error) {
	if o.cachedHostedZoneID != "" {
		return o.cachedHostedZoneID, nil
	}
	hostedZoneID, err := o.route53.DomainHostedZoneID(domainName)
	if err != nil {
		return "", fmt.Errorf("get hosted zone ID for domain %s: %w", domainName, err)
	}
	return hostedZoneID, nil
}

// RecommendActions returns a list of suggested additional commands users can run after successfully executing this command.
func (o *initAppOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Run %s to add a new service or job to your application.", color.HighlightCode("copilot init")),
	})
	return nil
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
	o.name = appName
	return nil
}

func (o *initAppOpts) askSelectExistingAppName(existingApps []*config.Application) error {
	var names []string
	for _, p := range existingApps {
		names = append(names, p.Name)
	}
	name, err := o.prompt.SelectOne(
		fmt.Sprintf("Which %s do you want to add a new service or job to?", color.Emphasize("existing application")),
		appInitNameHelpPrompt,
		names,
		prompt.WithFinalMessage("Application name:"))
	if err != nil {
		return fmt.Errorf("prompt select application name: %w", err)
	}
	o.name = name
	return nil
}

// buildAppInitCommand builds the command for creating a new application.
func buildAppInitCommand() *cobra.Command {
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
				opts.name = args[0]
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVar(&vars.domainName, domainNameFlag, "", domainNameFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	return cmd
}
