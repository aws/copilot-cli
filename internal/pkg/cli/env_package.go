// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/spf13/cobra"
)

const (
	envCFNTemplateNameFmt              = "%s.env.yml"
	envCFNTemplateConfigurationNameFmt = "%s.env.params.json"
)

type packageEnvVars struct {
	envName        string
	appName        string
	outputDir      string
	uploadAssets   bool
	forceNewUpdate bool
}

type discardFile struct{}

func (df discardFile) Write(p []byte) (n int, err error) {
	return io.Discard.Write(p)
}

func (df discardFile) Close() error {
	return nil //noop
}

type packageEnvOpts struct {
	packageEnvVars

	// Dependencies.
	cfgStore     store
	ws           wsEnvironmentReader
	sel          wsEnvironmentSelector
	caller       identityService
	fs           afero.Fs
	tplWriter    io.WriteCloser
	paramsWriter io.WriteCloser

	newInterpolator func(appName, envName string) interpolator
	newEnvDeployer  func() (envPackager, error)

	// Cached variables.
	appCfg *config.Application
	envCfg *config.Environment
}

func newPackageEnvOpts(vars packageEnvVars) (*packageEnvOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env package"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %v", err)
	}
	cfgStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	opts := &packageEnvOpts{
		packageEnvVars: vars,

		cfgStore:     cfgStore,
		ws:           ws,
		sel:          selector.NewLocalEnvironmentSelector(prompt.New(), cfgStore, ws),
		caller:       identity.New(defaultSess),
		fs:           &afero.Afero{Fs: afero.NewOsFs()},
		tplWriter:    os.Stdout,
		paramsWriter: discardFile{},

		newInterpolator: func(appName, envName string) interpolator {
			return manifest.NewInterpolator(appName, envName)
		},
	}
	opts.newEnvDeployer = func() (envPackager, error) {
		appCfg, err := opts.getAppCfg()
		if err != nil {
			return nil, err
		}
		envCfg, err := opts.getEnvCfg()
		if err != nil {
			return nil, err
		}
		return deploy.NewEnvDeployer(&deploy.NewEnvDeployerInput{
			App:             appCfg,
			Env:             envCfg,
			SessionProvider: sessProvider,
		})
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *packageEnvOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *packageEnvOpts) Ask() error {
	if o.appName == "" {
		// This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}

	if _, err := o.getAppCfg(); err != nil {
		return err
	}
	return o.validateOrAskEnvName()
}

// Execute prints the CloudFormation configuration for the environment.
func (o *packageEnvOpts) Execute() error {
	rawMft, err := o.ws.ReadEnvironmentManifest(o.envName)
	if err != nil {
		return fmt.Errorf("read manifest for environment %q: %w", o.envName, err)
	}
	mft, err := environmentManifest(o.envName, rawMft, o.newInterpolator(o.appName, o.envName))
	if err != nil {
		return err
	}
	principal, err := o.caller.Get()
	if err != nil {
		return fmt.Errorf("get caller principal identity: %v", err)
	}
	deployer, err := o.newEnvDeployer()
	if err != nil {
		return err
	}

	var urls map[string]string
	if o.uploadAssets {
		urls, err = deployer.UploadArtifacts()
		if err != nil {
			return fmt.Errorf("upload assets for environment %q: %v", o.envName, err)
		}
	}
	res, err := deployer.GenerateCloudFormationTemplate(&deploy.DeployEnvironmentInput{
		RootUserARN:         principal.RootUserARN,
		CustomResourcesURLs: urls,
		Manifest:            mft,
		RawManifest:         rawMft,
		ForceNewUpdate:      o.forceNewUpdate,
	})
	if err != nil {
		return fmt.Errorf("generate CloudFormation template from environment %q manifest: %v", o.envName, err)
	}
	if err := o.setWriters(); err != nil {
		return err
	}
	if err := o.writeAndClose(o.tplWriter, res.Template); err != nil {
		return err
	}
	return o.writeAndClose(o.paramsWriter, res.Parameters)
}

func (o *packageEnvOpts) getAppCfg() (*config.Application, error) {
	if o.appCfg != nil {
		return o.appCfg, nil
	}
	cfg, err := o.cfgStore.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %q configuration: %w", o.appName, err)
	}
	o.appCfg = cfg
	return o.appCfg, nil
}

func (o *packageEnvOpts) getEnvCfg() (*config.Environment, error) {
	if o.envCfg != nil {
		return o.envCfg, nil
	}
	cfg, err := o.cfgStore.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %q in application %q: %w", o.envName, o.appName, err)
	}
	o.envCfg = cfg
	return o.envCfg, nil
}

func (o *packageEnvOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		if _, err := o.getEnvCfg(); err != nil {
			log.Errorf("It seems like environment %s is not added in application %s yet. Have you run %s?\n",
				o.envName, o.appName, color.HighlightCode("copilot env init"))
			return err
		}
		return nil
	}

	name, err := o.sel.LocalEnvironment("Select an environment manifest from your workspace", "")
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *packageEnvOpts) setWriters() error {
	if o.outputDir == "" {
		return nil
	}
	if err := o.fs.MkdirAll(o.outputDir, 0755); err != nil {
		return fmt.Errorf("create directory %q: %w", o.outputDir, err)
	}

	path := filepath.Join(o.outputDir, fmt.Sprintf(envCFNTemplateNameFmt, o.envName))
	tplFile, err := o.fs.Create(path)
	if err != nil {
		return fmt.Errorf("create file at %q: %w", path, err)
	}
	path = filepath.Join(o.outputDir, fmt.Sprintf(envCFNTemplateConfigurationNameFmt, o.envName))
	paramsFile, err := o.fs.Create(path)
	if err != nil {
		return fmt.Errorf("create file at %q: %w", path, err)
	}

	o.tplWriter = tplFile
	o.paramsWriter = paramsFile
	return nil
}

func (o *packageEnvOpts) writeAndClose(wc io.WriteCloser, dat string) error {
	if _, err := wc.Write([]byte(dat)); err != nil {
		return err
	}
	return wc.Close()
}

// buildEnvPkgCmd builds the command for printing an environment CloudFormation stack configuration.
func buildEnvPkgCmd() *cobra.Command {
	vars := packageEnvVars{}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Print the AWS CloudFormation template of an environment.",
		Long:  `Print the CloudFormation stack template and configuration used to deploy an environment.`,
		Example: `
  Print the CloudFormation template for the "prod" environment.
  /code $ copilot env package -n prod --upload-assets

  Write the CloudFormation template and configuration to a "infrastructure/" sub-directory instead of stdout.
  /startcodeblock
  $ copilot env package -n test --output-dir ./infrastructure --upload-assets
  $ ls ./infrastructure
  test.env.yml      test.env.params.json
  /endcodeblock`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageEnvOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.envName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	cmd.Flags().BoolVar(&vars.uploadAssets, uploadAssetsFlag, false, uploadAssetsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceEnvDeployFlagDescription)
	return cmd
}
