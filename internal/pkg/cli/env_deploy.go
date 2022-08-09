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
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployEnvVars struct {
	appName        string
	name           string
	forceNewUpdate bool
}

type deployEnvOpts struct {
	deployEnvVars

	// Dependencies.
	store           store
	sessionProvider *sessions.Provider

	// Dependencies to ask.
	sel wsEnvironmentSelector

	// Dependencies to execute.
	ws              wsEnvironmentReader
	identity        identityService
	newInterpolator func(app, env string) interpolator
	newEnvDeployer  func() (envDeployer, error)
	newEnvDescriber func() (envDescriber, error)

	// Cached variables.
	targetApp *config.Application
	targetEnv *config.Environment
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

		store:           store,
		sessionProvider: sessProvider,
		sel:             selector.NewLocalEnvironmentSelector(prompt.New(), store, ws),

		ws:              ws,
		identity:        identity.New(defaultSess),
		newInterpolator: newManifestInterpolator,
	}
	opts.newEnvDeployer = func() (envDeployer, error) {
		return newEnvDeployer(opts)
	}
	opts.newEnvDescriber = func() (envDescriber, error) {
		envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         opts.appName,
			Env:         opts.name,
			ConfigStore: opts.store,
		})
		if err != nil {
			return nil, err
		}
		return envDescriber, nil
	}
	return opts, nil
}

func newEnvDeployer(opts *deployEnvOpts) (envDeployer, error) {
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
		SessionProvider: opts.sessionProvider,
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

func (o *deployEnvOpts) isManagedCDNEnabled(mft *manifest.Environment) bool {
	return mft.CDNEnabled() && mft.HTTPConfig.Public.Certificates == nil && o.targetApp.Domain != ""
}

// Execute deploys an environment given a manifest.
func (o *deployEnvOpts) Execute() error {
	rawMft, err := o.ws.ReadEnvironmentManifest(o.name)
	if err != nil {
		return fmt.Errorf("read manifest for environment %q: %w", o.name, err)
	}
	mft, err := environmentManifest(o.name, rawMft, o.newInterpolator(o.appName, o.name))
	if err != nil {
		return err
	}
	if o.isManagedCDNEnabled(mft) {
		describer, err := o.newEnvDescriber()
		if err != nil {
			return err
		}
		// With managed domain, if the customer isn't using `alias` the A-records are inserted in the service stack as each service domain is unique.
		// However, when clients enable CloudFront, they would need to update all their existing records to now point to the distribution.
		// Hence, we force users to use `alias` and let the records be written in the environment stack instead.
		if err := describer.ValidateCFServiceDomainAliases(); err != nil {
			return err
		}
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
		ForceNewUpdate:      o.forceNewUpdate,
		RawManifest:         rawMft,
	}); err != nil {
		var errEmptyChangeSet *awscfn.ErrChangeSetEmpty
		if errors.As(err, &errEmptyChangeSet) {
			log.Errorf(`Your update does not introduce immediate resource changes. 
This may be because the resources are not created until they are deemed 
necessary by a service deployment.

In this case, you can run %s to push a modified template, even if there are no immediate changes.
`, color.HighlightCode("copilot env deploy --force"))
		}
		return fmt.Errorf("deploy environment %s: %w", o.name, err)
	}
	return nil
}

func environmentManifest(envName string, rawMft []byte, transformer interpolator) (*manifest.Environment, error) {
	interpolated, err := transformer.Interpolate(string(rawMft))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %q manifest: %w", envName, err)
	}
	mft, err := manifest.UnmarshalEnvironment([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal environment manifest for %q: %w", envName, err)
	}
	if err := mft.Validate(); err != nil {
		return nil, fmt.Errorf("validate environment manifest for %q: %w", envName, err)
	}
	return mft, nil
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
	return cmd
}
