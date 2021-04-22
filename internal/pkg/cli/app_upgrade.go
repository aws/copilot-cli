// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	fmtAppUpgradeStart    = "Upgrading application %s from version %s to version %s."
	fmtAppUpgradeFailed   = "Failed to upgrade application %s's template to version %s.\n"
	fmtAppUpgradeComplete = "Upgraded application %s's template to version %s.\n"
)

// appUpgradeVars holds flag values.
type appUpgradeVars struct {
	name string
}

// appUpgradeOpts represents the app upgrade command and holds the necessary data
// and clients to execute the command.
type appUpgradeOpts struct {
	appUpgradeVars

	store         store
	prog          progress
	versionGetter versionGetter
	identity      identityService
	upgrader      appUpgrader
}

func newAppUpgradeOpts(vars appUpgradeVars) (*appUpgradeOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %v", err)
	}
	sess, err := sessions.NewProvider().Default()
	if err != nil {
		return nil, err
	}
	d, err := describe.NewAppDescriber(vars.name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %v", vars.name, err)
	}
	return &appUpgradeOpts{
		appUpgradeVars: vars,
		store:          store,
		identity:       identity.New(sess),
		prog:           termprogress.NewSpinner(log.DiagnosticWriter),
		versionGetter:  d,
		upgrader:       cloudformation.New(sess),
	}, nil
}

// Validate is a no-op for this command.
func (o *appUpgradeOpts) Validate() error {
	return nil
}

// Ask prompts is a no-op for this command.
func (o *appUpgradeOpts) Ask() error {
	return nil
}

// Execute updates the cloudformation stack as well as the stackset of an application to the latest version.
// If any stack is busy updating, it spins and waits until the stack can be updated.
func (o *appUpgradeOpts) Execute() error {
	version, err := o.versionGetter.Version()
	if err != nil {
		return fmt.Errorf("get template version of application %s: %v", o.name, err)
	}
	if !shouldUpgradeApp(o.name, version) {
		return nil
	}
	app, err := o.store.GetApplication(o.name)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.name, err)
	}
	o.prog.Start(fmt.Sprintf(fmtAppUpgradeStart, color.HighlightUserInput(o.name), color.Emphasize(version), color.Emphasize(deploy.LatestAppTemplateVersion)))
	defer func() {
		if err != nil {
			o.prog.Stop(log.Serrorf(fmtAppUpgradeFailed, color.HighlightUserInput(o.name), color.Emphasize(deploy.LatestAppTemplateVersion)))
			return
		}
		o.prog.Stop(log.Ssuccessf(fmtAppUpgradeComplete, color.HighlightUserInput(o.name), color.Emphasize(deploy.LatestAppTemplateVersion)))
	}()
	err = o.upgradeApplication(app, version, deploy.LatestAppTemplateVersion)
	if err != nil {
		return err
	}
	return nil
}

// RecommendedActions is a no-op for this command.
func (o *appUpgradeOpts) RecommendedActions() []string {
	return nil
}

func shouldUpgradeApp(appName string, version string) bool {
	diff := semver.Compare(version, deploy.LatestAppTemplateVersion)
	if diff < 0 {
		// Newer version available.
		return true
	}

	msg := fmt.Sprintf("Application %s is already on the latest version %s, skip upgrade.", appName, deploy.LatestAppTemplateVersion)
	if diff > 0 {
		// It's possible that a teammate used a different version of the CLI to upgrade the application
		// to a newer version. And the current user is on an older version of the CLI.
		// In this situation we notify them they should update the CLI.
		msg = fmt.Sprintf(`Skip upgrading application %s to version %s since it's on version %s. 
Are you using the latest version of AWS Copilot?`, appName, deploy.LatestAppTemplateVersion, version)
	}
	log.Debugln(msg)
	return false
}

func (o *appUpgradeOpts) upgradeApplication(app *config.Application, fromVersion, toVersion string) error {
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
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

// buildAppUpgradeCmd builds the command to update an application to the latest version.
func buildAppUpgradeCmd() *cobra.Command {
	vars := appUpgradeVars{}
	cmd := &cobra.Command{
		Use:    "upgrade",
		Short:  "Upgrades the template of an application to the latest version.",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newAppUpgradeOpts(vars)
			if err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
