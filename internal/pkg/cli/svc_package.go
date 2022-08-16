// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	svcPackageSvcNamePrompt = "Which service would you like to generate a CloudFormation template for?"
	svcPackageEnvNamePrompt = "Which environment would you like to package this stack for?"
)

type packageSvcVars struct {
	name         string
	envName      string
	appName      string
	tag          string
	outputDir    string
	uploadAssets bool

	// To facilitate unit tests.
	clientConfigured bool
}

type packageSvcOpts struct {
	packageSvcVars

	// Interfaces to interact with dependencies.
	ws                   wsWlDirReader
	fs                   afero.Fs
	store                store
	templateWriter       io.WriteCloser
	paramsWriter         io.WriteCloser
	addonsWriter         io.WriteCloser
	runner               execRunner
	sessProvider         *sessions.Provider
	sel                  wsSelector
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
	newStackGenerator    func(*packageSvcOpts) (workloadStackGenerator, error)
	envFeaturesDescriber versionCompatibilityChecker

	// cached variables
	targetApp         *config.Application
	targetEnv         *config.Environment
	envSess           *session.Session
	appliedDynamicMft manifest.DynamicWorkload
	rootUserARN       string
}

func newPackageSvcOpts(vars packageSvcVars) (*packageSvcOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc package"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	prompter := prompt.New()
	opts := &packageSvcOpts{
		packageSvcVars:    vars,
		store:             store,
		ws:                ws,
		fs:                &afero.Afero{Fs: afero.NewOsFs()},
		unmarshal:         manifest.UnmarshalWorkload,
		runner:            exec.NewCmd(),
		sel:               selector.NewLocalWorkloadSelector(prompter, store, ws),
		templateWriter:    os.Stdout,
		paramsWriter:      discardFile{},
		addonsWriter:      discardFile{},
		newInterpolator:   newManifestInterpolator,
		sessProvider:      sessProvider,
		newStackGenerator: newWorkloadStackGenerator,
	}
	return opts, nil
}

func newWorkloadStackGenerator(o *packageSvcOpts) (workloadStackGenerator, error) {
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
	}
	targetEnv, err := o.getTargetEnv()
	if err != nil {
		return nil, err
	}
	raw, err := o.ws.ReadWorkloadManifest(o.name)
	if err != nil {
		return nil, fmt.Errorf("read manifest file for %s: %w", o.name, err)
	}

	content := o.appliedDynamicMft.Manifest()
	var deployer workloadStackGenerator
	in := clideploy.WorkloadDeployerInput{
		SessionProvider: o.sessProvider,
		Name:            o.name,
		App:             targetApp,
		Env:             targetEnv,
		ImageTag:        o.tag,
		Mft:             content,
		RawMft:          raw,
	}
	switch t := content.(type) {
	case *manifest.LoadBalancedWebService:
		deployer, err = clideploy.NewLBWSDeployer(&in)
	case *manifest.BackendService:
		deployer, err = clideploy.NewBackendDeployer(&in)
	case *manifest.RequestDrivenWebService:
		deployer, err = clideploy.NewRDWSDeployer(&in)
	case *manifest.WorkerService:
		deployer, err = clideploy.NewWorkerSvcDeployer(&in)
	case *manifest.ScheduledJob:
		deployer, err = clideploy.NewJobDeployer(&in)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("initiate workload template generator: %w", err)
	}
	return deployer, nil
}

// Validate returns an error for any invalid optional flags.
func (o *packageSvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *packageSvcOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.getTargetApp(); err != nil {
			return err
		}
	} else {
		// NOTE: This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}
	if err := o.validateOrAskSvcName(); err != nil {
		return err
	}
	if err := o.validateOrAskEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageSvcOpts) Execute() error {
	if !o.clientConfigured {
		if err := o.configureClients(); err != nil {
			return err
		}
	}
	if o.outputDir != "" {
		if err := o.setOutputFileWriters(); err != nil {
			return err
		}
	}
	targetEnv, err := o.getTargetEnv()
	if err != nil {
		return nil
	}
	gen, err := o.getStackGenerator(targetEnv)
	if err != nil {
		return err
	}
	stack, err := o.getWorkloadStack(gen)
	if err != nil {
		return err
	}
	if err := o.writeAndClose(o.templateWriter, stack.template); err != nil {
		return err
	}
	if err := o.writeAndClose(o.paramsWriter, stack.parameters); err != nil {
		return err
	}
	addonsTemplate, err := gen.AddonsTemplate()
	switch {
	case err != nil:
		return fmt.Errorf("retrieve addons template: %w", err)
	case addonsTemplate == "":
		return nil
	}
	// Addons template won't show up without setting --output-dir flag.
	if o.outputDir != "" {
		if err := o.setAddonsFileWriter(); err != nil {
			return err
		}
	}
	return o.writeAndClose(o.addonsWriter, addonsTemplate)
}

func (o *packageSvcOpts) validateOrAskSvcName() error {
	if o.name != "" {
		names, err := o.ws.ListServices()
		if err != nil {
			return fmt.Errorf("list services in the workspace: %w", err)
		}
		if !contains(o.name, names) {
			return fmt.Errorf("service '%s' does not exist in the workspace", o.name)
		}
		return nil
	}

	name, err := o.sel.Service(svcPackageSvcNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *packageSvcOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		_, err := o.getTargetEnv()
		return err
	}

	name, err := o.sel.Environment(svcPackageEnvNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *packageSvcOpts) configureClients() error {
	o.tag = imageTagFromGit(o.runner, o.tag) // Best effort assign git tag.
	// client to retrieve an application's resources created with CloudFormation.
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	targetEnv, err := o.getTargetEnv()
	if err != nil {
		return err
	}
	envSess, err := o.sessProvider.FromRole(targetEnv.ManagerRoleARN, targetEnv.Region)
	if err != nil {
		return err
	}
	o.envSess = envSess
	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return err
	}
	o.envFeaturesDescriber = envDescriber
	return nil
}

type cfnStackConfig struct {
	template   string
	parameters string
}

func (o *packageSvcOpts) getStackGenerator(env *config.Environment) (workloadStackGenerator, error) {
	mft, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		interpolator: o.newInterpolator(o.appName, o.envName),
		ws:           o.ws,
		unmarshal:    o.unmarshal,
		sess:         o.envSess,
	})
	if err != nil {
		return nil, err
	}
	o.appliedDynamicMft = mft
	return o.newStackGenerator(o)
}

// getWorkloadStack returns the CloudFormation stack's template and its parameters for the service.
func (o *packageSvcOpts) getWorkloadStack(generator workloadStackGenerator) (*cfnStackConfig, error) {
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
	}
	uploadOut := clideploy.UploadArtifactsOutput{
		ImageDigest: aws.String(""),
	}
	if o.uploadAssets {
		out, err := generator.UploadArtifacts()
		if err != nil {
			return nil, fmt.Errorf("upload resources required for deployment for %s: %w", o.name, err)
		}
		uploadOut = *out
	}
	output, err := generator.GenerateCloudFormationTemplate(&clideploy.GenerateCloudFormationTemplateInput{
		StackRuntimeConfiguration: clideploy.StackRuntimeConfiguration{
			RootUserARN:        o.rootUserARN,
			Tags:               targetApp.Tags,
			ImageDigest:        uploadOut.ImageDigest,
			EnvFileARN:         uploadOut.EnvFileARN,
			AddonsURL:          uploadOut.AddonsURL,
			CustomResourceURLs: uploadOut.CustomResourceURLs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate workload %s template against environment %s: %w", o.name, o.envName, err)
	}
	return &cfnStackConfig{
		template:   output.Template,
		parameters: output.Parameters}, nil
}

// setOutputFileWriters creates the output directory, and updates the template and param writers to file writers in the directory.
func (o *packageSvcOpts) setOutputFileWriters() error {
	if err := o.fs.MkdirAll(o.outputDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", o.outputDir, err)
	}

	templatePath := filepath.Join(o.outputDir,
		fmt.Sprintf(deploy.WorkloadCfnTemplateNameFormat, o.name, o.envName))
	templateFile, err := o.fs.Create(templatePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", templatePath, err)
	}
	o.templateWriter = templateFile

	paramsPath := filepath.Join(o.outputDir,
		fmt.Sprintf(deploy.WorkloadCfnTemplateConfigurationNameFormat, o.name, o.envName))
	paramsFile, err := o.fs.Create(paramsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", paramsPath, err)
	}
	o.paramsWriter = paramsFile

	return nil
}

func (o *packageSvcOpts) setAddonsFileWriter() error {
	addonsPath := filepath.Join(o.outputDir,
		fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, o.name))
	addonsFile, err := o.fs.Create(addonsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", addonsPath, err)
	}
	o.addonsWriter = addonsFile

	return nil
}

func (o *packageSvcOpts) getTargetApp() (*config.Application, error) {
	if o.targetApp != nil {
		return o.targetApp, nil
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s configuration: %w", o.appName, err)
	}
	o.targetApp = app
	return o.targetApp, nil
}

func (o *packageSvcOpts) getTargetEnv() (*config.Environment, error) {
	if o.targetEnv != nil {
		return o.targetEnv, nil
	}
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", o.envName, err)
	}
	o.targetEnv = env
	return o.targetEnv, nil
}

func (o *packageSvcOpts) writeAndClose(wc io.WriteCloser, dat string) error {
	if _, err := wc.Write([]byte(dat)); err != nil {
		return err
	}
	return wc.Close()
}

// RecommendActions suggests recommended actions before the packaged template is used for deployment.
func (o *packageSvcOpts) RecommendActions() error {
	return validateWorkloadManifestCompatibilityWithEnv(o.ws, o.envFeaturesDescriber, o.appliedDynamicMft, o.envName)
}

func contains(s string, items []string) bool {
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}

// buildSvcPackageCmd builds the command for printing a service's CloudFormation template.
func buildSvcPackageCmd() *cobra.Command {
	vars := packageSvcVars{}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Print the AWS CloudFormation template of a service.",
		Long:  `Print the CloudFormation template used to deploy a service to an environment.`,
		Example: `
  Print the CloudFormation template for the "frontend" service parametrized for the "test" environment.
  /code $ copilot svc package -n frontend -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of stdout.
  /startcodeblock
  $ copilot svc package -n frontend -e test --output-dir ./infrastructure
  $ ls ./infrastructure
  frontend-test.stack.yml      frontend-test.params.json
  /endcodeblock`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageSvcOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	cmd.Flags().BoolVar(&vars.uploadAssets, uploadAssetsFlag, false, uploadAssetsFlagDescription)
	return cmd
}
