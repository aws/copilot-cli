// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/route53"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	fmtAppUpgradeStart    = "Upgrading application %s from version %s to version %s.\n"
	fmtAppUpgradeFailed   = "Failed to upgrade application %s's template to version %s.\n"
	fmtAppUpgradeComplete = "Upgraded application %s's template to version %s.\n"

	appUpgradeNamePrompt     = "Which application would you like to upgrade?"
	appUpgradeNameHelpPrompt = "An application is a collection of related services."
)

// appUpgradeVars holds flag values.
type appUpgradeVars struct {
	name string
}

// appUpgradeOpts represents the app upgrade command and holds the necessary data
// and clients to execute the command.
type appUpgradeOpts struct {
	appUpgradeVars

	store    store
	route53  domainHostedZoneGetter
	sel      appSelector
	identity identityService
	upgrader appUpgrader

	newVersionGetter func(string) (versionGetter, error)

	templateVersion string // Overridden in tests.
}

func newAppUpgradeOpts(vars appUpgradeVars) (*appUpgradeOpts, error) {
	sess, err := sessions.ImmutableProvider(sessions.UserAgentExtras("app upgrade")).Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(sess), ssm.New(sess), aws.StringValue(sess.Config.Region))
	return &appUpgradeOpts{
		appUpgradeVars: vars,
		store:          store,
		identity:       identity.New(sess),
		route53:        route53.New(sess),
		sel:            selector.NewAppEnvSelector(prompt.New(), store),
		upgrader:       cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr)),
		newVersionGetter: func(appName string) (versionGetter, error) {
			d, err := describe.NewAppDescriber(appName)
			if err != nil {
				return d, fmt.Errorf("new describer for application %q: %w", appName, err)
			}
			return d, nil
		},
		templateVersion: version.LatestTemplateVersion(),
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *appUpgradeOpts) Validate() error {
	if o.name != "" {
		_, err := o.store.GetApplication(o.name)
		if err != nil {
			return fmt.Errorf("get application %s: %w", o.name, err)
		}
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *appUpgradeOpts) Ask() error {
	if err := o.askName(); err != nil {
		return err
	}
	return nil
}

// Execute updates the cloudformation stack as well as the stackset of an application to the latest version.
// If any stack is busy updating, it spins and waits until the stack can be updated.
func (o *appUpgradeOpts) Execute() error {
	vg, err := o.newVersionGetter(o.name)
	if err != nil {
		return err
	}

	appVersion, err := vg.Version()
	if err != nil {
		return fmt.Errorf("get template version of application %s: %v", o.name, err)
	}
	if !o.shouldUpgradeApp(appVersion) {
		return nil
	}
	app, err := o.store.GetApplication(o.name)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.name, err)
	}
	log.Infof(fmtAppUpgradeStart, color.HighlightUserInput(o.name), color.Emphasize(appVersion), color.Emphasize(o.templateVersion))
	defer func() {
		if err != nil {
			log.Errorf(fmtAppUpgradeFailed, color.HighlightUserInput(o.name), color.Emphasize(o.templateVersion))
			return
		}
		log.Successf(fmtAppUpgradeComplete, color.HighlightUserInput(o.name), color.Emphasize(o.templateVersion))
	}()
	err = o.upgradeApplication(app, appVersion, o.templateVersion)
	if err != nil {
		return err
	}
	return nil
}

func (o *appUpgradeOpts) askName() error {
	if o.name != "" {
		return nil
	}
	name, err := o.sel.Application(appUpgradeNamePrompt, appUpgradeNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.name = name
	return nil
}

func (o *appUpgradeOpts) shouldUpgradeApp(appVersion string) bool {
	diff := semver.Compare(appVersion, o.templateVersion)
	if diff < 0 {
		// Newer version available.
		return true
	}

	msg := fmt.Sprintf("Application %s is already on the latest version %s, skip upgrade.", o.name, o.templateVersion)
	if diff > 0 {
		// It's possible that a teammate used a different version of the CLI to upgrade the application
		// to a newer version. And the current user is on an older version of the CLI.
		// In this situation we notify them they should update the CLI.
		msg = fmt.Sprintf(`Skip upgrading application %s to version %s since it's on version %s. 
Are you using the latest version of AWS Copilot?`, o.name, o.templateVersion, appVersion)
	}
	log.Debugln(msg)
	return false
}

func (o *appUpgradeOpts) upgradeApplication(app *config.Application, fromVersion, toVersion string) error {
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	// Upgrade SSM Parameter Store record.
	if err := o.upgradeAppSSMStore(app); err != nil {
		return err
	}
	// Upgrade app CloudFormation resources.
	if err := o.upgrader.UpgradeApplication(&deploy.CreateAppInput{
		Name:               o.name,
		AccountID:          caller.Account,
		DomainName:         app.Domain,
		DomainHostedZoneID: app.DomainHostedZoneID,
		Version:            toVersion,
	}); err != nil {
		return fmt.Errorf("upgrade application %s from version %s to version %s: %v", app.Name, fromVersion, toVersion, err)
	}
	return nil
}

func (o *appUpgradeOpts) upgradeAppSSMStore(app *config.Application) error {
	if app.Domain != "" && app.DomainHostedZoneID == "" {
		hostedZoneID, err := o.route53.PublicDomainHostedZoneID(app.Domain)
		if err != nil {
			return fmt.Errorf("get hosted zone ID for domain %s: %w", app.Domain, err)
		}
		app.DomainHostedZoneID = hostedZoneID
	}
	if err := o.store.UpdateApplication(app); err != nil {
		return fmt.Errorf("update application %s: %w", app.Name, err)
	}
	return nil
}

// buildAppUpgradeCmd builds the command to update an application to the latest version.
func buildAppUpgradeCmd() *cobra.Command {
	vars := appUpgradeVars{}
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrades the template of an application to the latest version.",
		Example: `
    Upgrade the application "my-app" to the latest version
    /code $ copilot app upgrade -n my-app`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newAppUpgradeOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
