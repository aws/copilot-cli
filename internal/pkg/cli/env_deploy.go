// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const continueDeploymentPrompt = "Continue with the deployment?"

type deployEnvVars struct {
	appName           string
	name              string
	forceNewUpdate    bool
	disableRollback   bool
	showDiff          bool
	skipDiffPrompt    bool
	allowEnvDowngrade bool
	detach            bool
}

type deployEnvOpts struct {
	deployEnvVars

	// Dependencies.
	store           store
	sessionProvider *sessions.Provider

	// Dependencies to ask.
	sel    wsEnvironmentSelector
	prompt prompter

	// Dependencies to execute.
	fs                  afero.Fs
	ws                  wsEnvironmentReader
	identity            identityService
	newInterpolator     func(app, env string) interpolator
	newEnvVersionGetter func(appName, envName string) (versionGetter, error)
	newEnvDeployer      func() (envDeployer, error)

	// Cached variables.
	targetApp *config.Application
	targetEnv *config.Environment

	// Overridden in tests.
	templateVersion string
}

func newEnvDeployOpts(vars deployEnvVars) (*deployEnvOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env deploy"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}
	prompter := prompt.New()
	opts := &deployEnvOpts{
		deployEnvVars: vars,

		store:           store,
		sessionProvider: sessProvider,
		sel:             selector.NewLocalEnvironmentSelector(prompter, store, ws),
		newEnvVersionGetter: func(appName, envName string) (versionGetter, error) {
			return describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
				App:         appName,
				Env:         envName,
				ConfigStore: store,
			})
		},
		prompt: prompter,

		fs:              fs,
		ws:              ws,
		identity:        identity.New(defaultSess),
		templateVersion: version.LatestTemplateVersion(),
		newInterpolator: newManifestInterpolator,
	}
	opts.newEnvDeployer = func() (envDeployer, error) {
		return newEnvDeployer(opts, ws)
	}
	return opts, nil
}

func newEnvDeployer(opts *deployEnvOpts, ws deploy.WorkspaceAddonsReaderPathGetter) (envDeployer, error) {
	app, err := opts.cachedTargetApp()
	if err != nil {
		return nil, err
	}
	env, err := opts.cachedTargetEnv()
	if err != nil {
		return nil, err
	}
	ovrdr, err := deploy.NewOverrider(opts.ws.EnvOverridesPath(), env.App, env.Name, opts.fs, opts.sessionProvider)
	if err != nil {
		return nil, err
	}
	return deploy.NewEnvDeployer(&deploy.NewEnvDeployerInput{
		App:             app,
		Env:             env,
		SessionProvider: opts.sessionProvider,
		ConfigStore:     opts.store,
		Workspace:       ws,
		Overrider:       ovrdr,
	})
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

func validateEnvVersion(vg versionGetter, name, templateVersion string) error {
	envVersion, err := vg.Version()
	if err != nil {
		return fmt.Errorf("get template version of environment %s: %w", name, err)
	}
	if envVersion == version.EnvTemplateBootstrap {
		// Allow update to bootstrap env stack anyway.
		return nil
	}
	if diff := semver.Compare(envVersion, templateVersion); diff > 0 {
		return &errCannotDowngradeEnvVersion{
			envName:         name,
			envVersion:      envVersion,
			templateVersion: templateVersion,
		}
	}
	return nil
}

// Execute deploys an environment given a manifest.
func (o *deployEnvOpts) Execute() error {
	if !o.allowEnvDowngrade {
		envVersionGetter, err := o.newEnvVersionGetter(o.appName, o.name)
		if err != nil {
			return err
		}
		if err := validateEnvVersion(envVersionGetter, o.name, o.templateVersion); err != nil {
			return err
		}
	}
	mft, interpolated, err := environmentManifest(o.name, o.ws, o.newInterpolator(o.appName, o.name))
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
	if err := deployer.Validate(mft); err != nil {
		return err
	}
	artifacts, err := deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload artifacts for environment %s: %w", o.name, err)
	}
	deployInput := &deploy.DeployEnvironmentInput{
		RootUserARN:         caller.RootUserARN,
		AddonsURL:           artifacts.AddonsURL,
		CustomResourcesURLs: artifacts.CustomResourceURLs,
		Manifest:            mft,
		RawManifest:         interpolated,
		PermissionsBoundary: o.targetApp.PermissionsBoundary,
		ForceNewUpdate:      o.forceNewUpdate,
		DisableRollback:     o.disableRollback,
		Version:             o.templateVersion,
		Detach:              o.detach,
	}
	if o.showDiff {
		contd, err := o.showDiffAndConfirmDeployment(deployer, deployInput)
		if err != nil {
			return err
		}
		if !contd {
			return nil
		}
	}
	err = deployer.DeployEnvironment(deployInput)
	if err == nil {
		if o.detach {
			return nil
		}
		log.Successf("Successfully deployed environment %s", o.name)
		return nil
	}
	var errStackDeletedOnInterrupt *deploycfn.ErrStackDeletedOnInterrupt
	var errStackUpdateCanceledOnInterrupt *deploycfn.ErrStackUpdateCanceledOnInterrupt
	var errEmptyChangeSet *awscfn.ErrChangeSetEmpty
	switch {
	case errors.As(err, &errStackDeletedOnInterrupt):
		return nil

	case errors.As(err, &errStackUpdateCanceledOnInterrupt):
		log.Successf("Successfully rolled back service %s to the previous configuration.\n", color.HighlightUserInput(o.name))
		return nil
	case errors.As(err, &errEmptyChangeSet):
		log.Errorf(`Your update does not introduce immediate resource changes. 
This may be because the resources are not created until they are deemed 
necessary by a service deployment.

In this case, you can run %s to push a modified template, even if there are no immediate changes.
`, color.HighlightCode("copilot env deploy --force"))
		if o.disableRollback {
			stackName := stack.NameForEnv(o.targetApp.Name, o.targetEnv.Name)
			rollbackCmd := fmt.Sprintf("aws cloudformation rollback-stack --stack-name %s --role-arn %s", stackName, o.targetEnv.ExecutionRoleARN)
			log.Infof(`It seems like you have disabled automatic stack rollback for this deployment.
To debug, you can visit the AWS console to inspect the errors.
After fixing the deployment, you can:
1. Run %s to rollback the deployment.
2. Run %s to make a new deployment.
`, color.HighlightCode(rollbackCmd), color.HighlightCode("copilot env deploy"))
		}
	}
	if errors.As(err, &errEmptyChangeSet) {
		return &errNoInfrastructureChanges{parentErr: err}
	}
	return fmt.Errorf("deploy environment %s: %w", o.name, err)
}

func environmentManifest(envName string, reader wsEnvironmentReader, transformer interpolator) (*manifest.Environment, string, error) {
	rawMft, err := reader.ReadEnvironmentManifest(envName)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest for environment %q: %w", envName, err)
	}
	interpolated, err := transformer.Interpolate(string(rawMft))
	if err != nil {
		return nil, "", fmt.Errorf("interpolate environment variables for %q manifest: %w", envName, err)
	}
	mft, err := manifest.UnmarshalEnvironment([]byte(interpolated))
	if err != nil {
		return nil, "", fmt.Errorf("unmarshal environment manifest for %q: %w", envName, err)
	}
	if err := mft.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate environment manifest for %q: %w", envName, err)
	}
	return mft, interpolated, nil
}

func (o *deployEnvOpts) showDiffAndConfirmDeployment(deployer envDeployer, input *deploy.DeployEnvironmentInput) (bool, error) {
	output, err := deployer.GenerateCloudFormationTemplate(input)
	if err != nil {
		return false, fmt.Errorf("generate the template for environment %q: %w", o.name, err)
	}
	if err := diff(deployer, output.Template, os.Stdout); err != nil {
		var errHasDiff *errHasDiff
		if !errors.As(err, &errHasDiff) {
			return false, fmt.Errorf("generate diff for environment %q: %w", o.name, err)
		}
	}
	if o.skipDiffPrompt {
		return true, nil
	}
	contd, err := o.prompt.Confirm(continueDeploymentPrompt, "")
	if err != nil {
		return false, fmt.Errorf("ask whether to continue with the deployment: %w", err)
	}
	return contd, nil
}

func (o *deployEnvOpts) validateOrAskEnvName() error {
	if o.name != "" {
		return o.validateEnvName()
	}
	name, err := o.sel.LocalEnvironment("Select an environment manifest from your workspace", "")
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) || errors.Is(err, selector.ErrLocalEnvsNotFound) {
			o.logManifestSuggestion("example")
		}
		return fmt.Errorf("select environment: %w", err)
	}
	o.name = name
	return nil
}

func (o *deployEnvOpts) validateEnvName() error {
	localEnvs, err := o.ws.ListEnvironments()
	if err != nil {
		o.logManifestSuggestion(o.name)
		return fmt.Errorf("list environments in workspace: %w", err)
	}
	for _, localEnv := range localEnvs {
		if o.name != localEnv {
			continue
		}
		if _, err := o.cachedTargetEnv(); err != nil {
			log.Errorf("It seems like environment %s is not added in application %s yet. Have you run %s?\n",
				o.name, o.appName, color.HighlightCode("copilot env init"))
			return err
		}
		return nil
	}
	o.logManifestSuggestion(o.name)
	return fmt.Errorf("environment manifest for %q is not found", o.name)
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

func (o *deployEnvOpts) logManifestSuggestion(envName string) {
	dir := filepath.Join("copilot", "environments", envName)
	log.Infof(`It looks like there are no environment manifests in your workspace.
To create a new manifest for an environment %q, please run:
1. Create the directories to store the manifest file:
   %s
2. Generate and write the manifest file:
   %s
`,
		envName,
		color.HighlightCode(fmt.Sprintf("mkdir -p %s", dir)),
		color.HighlightCode(fmt.Sprintf("copilot env show -n %s --manifest > %s", envName, filepath.Join(dir, "manifest.yml"))))
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
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceEnvDeployFlagDescription)
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.showDiff, diffFlag, false, diffFlagDescription)
	cmd.Flags().BoolVar(&vars.skipDiffPrompt, diffAutoApproveFlag, false, diffAutoApproveFlagDescription)
	cmd.Flags().BoolVar(&vars.allowEnvDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.detach, detachFlag, false, detachFlagDescription)
	return cmd
}
