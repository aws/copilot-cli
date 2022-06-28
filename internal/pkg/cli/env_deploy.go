// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployEnvVars struct {
	appName      string
	name         string
	isProduction bool
}

type deployEnvOpts struct {
	deployEnvVars

	// Dependencies.
	store store

	// Dependencies to ask.
	sel wsEnvironmentSelector

	// Dependencies to execute.
	ws             wsEnvironmentReader
	identity       identityService
	interpolator   interpolator
	newEnvDeployer func() (envDeployer, error)

	// Cached variables.
	targetApp *config.Application
	targetEnv *config.Environment

	// Functions to facilitate testing.
	unmarshalManifest func(in []byte) (*manifest.Environment, error)
}

func newEnvDeployOpts(vars deployEnvVars) (*deployEnvOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env deploy"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	opts := &deployEnvOpts{
		deployEnvVars: vars,

		store: store,
		sel:   selector.NewLocalEnvironmentSelector(prompt.New(), store, ws),

		ws:           ws,
		identity:     identity.New(defaultSess),
		interpolator: manifest.NewInterpolator(vars.appName, vars.name),

		unmarshalManifest: manifest.UnmarshalEnvironment,
	}
	opts.newEnvDeployer = func() (envDeployer, error) {
		app, err := opts.cachedTargetApp()
		if err != nil {
			return nil, err
		}
		env, err := opts.cachedTargetEnv()
		if err != nil {
			return nil, err
		}
		return deploy.NewEnvDeployer(&deploy.NewEnvDeployerInput{
			App:             app,
			Env:             env,
			SessionProvider: sessProvider,
		})
	}
	return opts, nil
}

// Validate is a no-op for this command.
func (o *deployEnvOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *deployEnvOpts) Ask() error {
	if o.appName == "" {
		// NOTE: This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}
	if _, err := o.cachedTargetApp(); err != nil {
		return err
	}
	return o.validateOrAskEnvName()
}

// Execute deploys an environment given a manifest.
func (o *deployEnvOpts) Execute() error {
	mft, err := o.environmentManifest()
	if err != nil {
		return err
	}
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	deployer, err := o.newEnvDeployer()
	if err != nil {
		return err
	}
	urls, err := deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload artifacts for environment %s: %w", o.name, err)
	}
	if err := deployer.DeployEnvironment(&deploy.DeployEnvironmentInput{
		RootUserARN:         caller.RootUserARN,
		CustomResourcesURLs: urls,
		Manifest:            mft,
	}); err != nil {
		return fmt.Errorf("deploy environment %s: %w", o.name, err)
	}
	return nil
}

func (o *deployEnvOpts) environmentManifest() (*manifest.Environment, error) {
	targetEnv, err := o.cachedTargetEnv()
	if err != nil {
		return nil, err
	}
	raw, err := o.ws.ReadEnvironmentManifest(targetEnv.Name)
	if err != nil {
		return nil, fmt.Errorf("read manifest for environment %s: %w", targetEnv.Name, err)
	}
	interpolated, err := o.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", targetEnv.Name, err)
	}
	mft, err := o.unmarshalManifest([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal environment manifest for %s: %w", targetEnv.Name, err)
	}
	if err := mft.Validate(); err != nil {
		return nil, fmt.Errorf("validate environment manifest for %s: %w", targetEnv.Name, err)
	}
	return mft, nil
}

func (o *deployEnvOpts) validateOrAskEnvName() error {
	if o.name != "" {
		if _, err := o.cachedTargetEnv(); err != nil {
			log.Errorf("It seems like environment %s is not added in application %s yet. Have you run %s?\n",
				o.name, o.appName, color.HighlightCode("copilot env init"))
			return err
		}
		return nil
	}
	name, err := o.sel.LocalEnvironment("Select an environment in your workspace", "")
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.name = name
	return nil
}

func (o *deployEnvOpts) cachedTargetEnv() (*config.Environment, error) {
	if o.targetEnv == nil {
		env, err := o.store.GetEnvironment(o.appName, o.name)
		if err != nil {
			return nil, fmt.Errorf("get environment %s in application %s: %w", o.name, o.appName, err)
		}
		o.targetEnv = env
	}
	return o.targetEnv, nil
}

func (o *deployEnvOpts) cachedTargetApp() (*config.Application, error) {
	if o.targetApp == nil {
		app, err := o.store.GetApplication(o.appName)
		if err != nil {
			return nil, fmt.Errorf("get application %s: %w", o.appName, err)
		}
		o.targetApp = app
	}
	return o.targetApp, nil
}

// buildEnvDeployCmd builds the command for deploying an environment given a manifest.
func buildEnvDeployCmd() *cobra.Command {
	vars := deployEnvVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an environment to an application.",
		Long:  "Deploys an environment to an application.",
		Example: `
Deploy an environment named "test".
/code $copilot env deploy --name test`,
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newEnvDeployOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", envFlagDescription)
	return cmd
}
