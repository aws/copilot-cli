// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/iam"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/afero"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ssm"

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
	fmtAppInitNamePrompt    = "What would you like to %s your application?"
	fmtAppInitNewNamePrompt = `Ok, let's create a new application then.
  What would you like to %s your application?`
	appInitNameHelpPrompt = "Services and jobs in the same application share the same VPC and ECS Cluster and services are discoverable via service discovery."
)

type initAppVars struct {
	name                string
	permissionsBoundary string
	domainName          string
	resourceTags        map[string]string
}

type initAppOpts struct {
	initAppVars

	identity             identityService
	store                applicationStore
	route53              domainHostedZoneGetter
	cfn                  appDeployer
	prompt               prompter
	prog                 progress
	iam                  policyLister
	iamRoleManager       roleManager
	isSessionFromEnvVars func() (bool, error)

	existingWorkspace func() (wsAppManager, error)
	newWorkspace      func(appName string) (wsAppManager, error)

	// Cached variables.
	cachedHostedZoneID string
}

func newInitAppOpts(vars initAppVars) (*initAppOpts, error) {
	sess, err := sessions.ImmutableProvider(sessions.UserAgentExtras("app init")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	fs := afero.NewOsFs()
	identity := identity.New(sess)
	iamClient := iam.New(sess)
	return &initAppOpts{
		initAppVars:    vars,
		identity:       identity,
		store:          config.NewSSMStore(identity, ssm.New(sess), aws.StringValue(sess.Config.Region)),
		route53:        route53.New(sess),
		cfn:            cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr)),
		prompt:         prompt.New(),
		prog:           termprogress.NewSpinner(log.DiagnosticWriter),
		iam:            iamClient,
		iamRoleManager: iamClient,
		isSessionFromEnvVars: func() (bool, error) {
			return sessions.AreCredsFromEnvVars(sess)
		},
		existingWorkspace: func() (wsAppManager, error) {
			return workspace.Use(fs)
		},
		newWorkspace: func(appName string) (wsAppManager, error) {
			return workspace.Create(appName, fs)
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
	if o.permissionsBoundary != "" {
		// Best effort to get the permission boundary name if ARN
		// (for example: arn:aws:iam::1234567890:policy/myPermissionsBoundaryPolicy).
		if arn.IsARN(o.permissionsBoundary) {
			parsed, err := arn.Parse(o.permissionsBoundary)
			if err != nil {
				return fmt.Errorf("parse permission boundary ARN: %w", err)
			}
			o.permissionsBoundary = strings.TrimPrefix(parsed.Resource, "policy/")
		}
		if err := o.validatePermBound(o.permissionsBoundary); err != nil {
			return err
		}
	}
	if o.domainName != "" {
		o.prog.Start(fmt.Sprintf("Validating ownership of %q", o.domainName))
		defer o.prog.Stop("")
		if err := validateDomainName(o.domainName); err != nil {
			return fmt.Errorf("domain name %s is invalid: %w", o.domainName, err)
		}
		o.warnIfDomainIsNotOwned()
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

	ws, err := o.existingWorkspace()
	if err == nil {
		// When there's a local application.
		summary, err := ws.Summary()
		if err == nil {
			if o.name == "" {
				log.Infoln(fmt.Sprintf(
					"Your workspace is registered to application %s.",
					color.HighlightUserInput(summary.Application)))
				if err := o.validateAppName(summary.Application); err != nil {
					return err
				}
				o.name = summary.Application
				return nil
			}
			if o.name != summary.Application {
				summaryPath := displayPath(summary.Path)
				if summaryPath == "" {
					summaryPath = summary.Path
				}

				log.Errorf(`Workspace is already registered with application %s instead of %s.
If you'd like to delete the application locally, you can delete the file at %s.
If you'd like to delete the application and all of its resources, run %s.
`,
					summary.Application,
					o.name,
					summaryPath,
					color.HighlightCode("copilot app delete"))
				return fmt.Errorf("workspace already registered with %s", summary.Application)
			}
		}

		var errNoAppSummary *workspace.ErrNoAssociatedApplication
		if !errors.As(err, &errNoAppSummary) {
			return err
		}
	}
	if !workspace.IsEmptyErr(err) {
		return err
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

	_, err = o.newWorkspace(o.name)
	if err != nil {
		return fmt.Errorf("create new workspace with application name %s: %w", o.name, err)
	}
	var hostedZoneID string
	if o.domainName != "" {
		hostedZoneID, err = o.domainHostedZoneID(o.domainName)
		if err != nil {
			return err
		}
	}
	err = o.cfn.DeployApp(&deploy.CreateAppInput{
		Name:                o.name,
		AccountID:           caller.Account,
		DomainName:          o.domainName,
		DomainHostedZoneID:  hostedZoneID,
		PermissionsBoundary: o.permissionsBoundary,
		AdditionalTags:      o.resourceTags,
		Version:             version.LatestTemplateVersion(),
	})
	if err != nil {
		return err
	}

	if err := o.store.CreateApplication(&config.Application{
		AccountID:           caller.Account,
		Name:                o.name,
		Domain:              o.domainName,
		DomainHostedZoneID:  hostedZoneID,
		PermissionsBoundary: o.permissionsBoundary,
		Tags:                o.resourceTags,
	}); err != nil {
		return err
	}
	log.Successf("The directory %s will hold service manifests for application %s.\n", color.HighlightResource(workspace.CopilotDirName), color.HighlightUserInput(o.name))
	log.Infoln()
	return nil
}

func (o *initAppOpts) validateAppName(name string) error {
	if err := validateAppNameString(name); err != nil {
		return err
	}
	app, err := o.store.GetApplication(name)
	if err == nil {
		if o.domainName != "" && app.Domain != o.domainName {
			return fmt.Errorf("application named %s already exists with a different domain name %s", name, app.Domain)
		}
		return nil
	}
	var noSuchAppErr *config.ErrNoSuchApplication
	if errors.As(err, &noSuchAppErr) {
		roleName := fmt.Sprintf("%s-adminrole", name)
		tags, err := o.iamRoleManager.ListRoleTags(roleName)
		// NOTE: This is a best-effort attempt to check if the app exists in other regions.
		// The error either indicates that the role does not exist, or not.
		// In the first case, it means that this is a valid app name, hence we don't error out.
		// In the second case, since this is a best-effort, we don't need to surface the error either.
		if err != nil {
			return nil
		}
		if _, hasTag := tags[deploy.AppTagKey]; hasTag {
			return &errAppAlreadyExistsInAccount{appName: name}
		}
		return &errStackSetAdminRoleExistsInAccount{
			appName:  name,
			roleName: roleName,
		}
	}
	return fmt.Errorf("get application %s: %w", name, err)
}

func (o *initAppOpts) validatePermBound(policyName string) error {
	IAMPolicies, err := o.iam.ListPolicyNames()
	if err != nil {
		return fmt.Errorf("list permissions boundary policies: %w", err)
	}
	for _, policy := range IAMPolicies {
		if policy == policyName {
			return nil
		}
	}
	return fmt.Errorf("IAM policy %q not found in this account", policyName)
}

func (o *initAppOpts) warnIfDomainIsNotOwned() {
	err := o.route53.ValidateDomainOwnership(o.domainName)
	if err == nil {
		return
	}
	var nsRecordsErr *route53.ErrUnmatchedNSRecords
	if errors.As(err, &nsRecordsErr) {
		o.prog.Stop("")
		log.Warningln(nsRecordsErr.RecommendActions())
	}
}

func (o *initAppOpts) domainHostedZoneID(domainName string) (string, error) {
	if o.cachedHostedZoneID != "" {
		return o.cachedHostedZoneID, nil
	}
	hostedZoneID, err := o.route53.PublicDomainHostedZoneID(domainName)
	if err != nil {
		return "", fmt.Errorf("get public hosted zone ID for domain %s: %w", domainName, err)
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
		validateAppNameString,
		prompt.WithFinalMessage("Application name:"))
	if err != nil {
		return fmt.Errorf("prompt get application name: %w", err)
	}
	if err := o.validateAppName(appName); err != nil {
		return err
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

type errAppAlreadyExistsInAccount struct {
	appName string
}

type errStackSetAdminRoleExistsInAccount struct {
	appName  string
	roleName string
}

func (e *errAppAlreadyExistsInAccount) Error() string {
	return fmt.Sprintf("application named %q already exists in another region", e.appName)
}

func (e *errAppAlreadyExistsInAccount) RecommendActions() string {
	return fmt.Sprintf(`If you want to create a new workspace reusing the existing application %s, please switch to the region where you created the application, and run %s.
If you'd like to recreate the application and all of its resources, please switch to the region where you created the application, and run %s.`, e.appName, color.HighlightCode("copilot app init"), color.HighlightCode("copilot app delete"))
}

func (e *errStackSetAdminRoleExistsInAccount) Error() string {
	return fmt.Sprintf("IAM admin role %q already exists in this account", e.roleName)
}

func (e *errStackSetAdminRoleExistsInAccount) RecommendActions() string {
	return fmt.Sprintf(`Copilot will create an IAM admin role named %s to manage the stack set of the application %s. 
You have an existing role with the exact same name in your account, which will collide with the role that Copilot creates.
Please create the application with a different name, so that the IAM role name does not collide.`, e.roleName, e.appName)
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
  Create a new application with an existing IAM policy as the permissions boundary for roles.
  /code $ copilot app init --permissions-boundary myPermissionsBoundaryPolicy
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
	cmd.Flags().StringVar(&vars.permissionsBoundary, permissionsBoundaryFlag, "", permissionsBoundaryFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	return cmd
}
