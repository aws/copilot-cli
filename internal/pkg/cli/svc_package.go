// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/aws-sdk-go/aws"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/addon"
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

var initPackageAddonsClient = func(o *packageSvcOpts) error {
	addonsClient, err := addon.New(o.name)
	if err != nil {
		return fmt.Errorf("new addons client: %w", err)
	}
	o.addonsClient = addonsClient
	return nil
}

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
	addonsClient     templater
	initAddonsClient func(*packageSvcOpts) error // Overridden in tests.
	ws               wsWlDirReader
	fs               afero.Fs
	store            store
	stackWriter      io.Writer
	paramsWriter     io.Writer
	addonsWriter     io.Writer
	runner           runner
	sessProvider     *sessions.Provider
	sel              wsSelector
	unmarshal        func([]byte) (manifest.WorkloadManifest, error)
	newInterpolator  func(app, env string) interpolator
	newTplGenerator  func(*packageSvcOpts) (workloadTemplateGenerator, error)

	// cached variables
	targetApp       *config.Application
	targetEnv       *config.Environment
	appliedManifest interface{}
	rootUserARN     string
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
		packageSvcVars:   vars,
		initAddonsClient: initPackageAddonsClient,
		store:            store,
		ws:               ws,
		fs:               &afero.Afero{Fs: afero.NewOsFs()},
		unmarshal:        manifest.UnmarshalWorkload,
		runner:           exec.NewCmd(),
		sel:              selector.NewWorkspaceSelect(prompter, store, ws),
		stackWriter:      os.Stdout,
		paramsWriter:     ioutil.Discard,
		addonsWriter:     ioutil.Discard,
		newInterpolator:  newManifestInterpolator,
		sessProvider:     sessProvider,
		newTplGenerator:  newWkldTplGenerator,
	}
	return opts, nil
}

func newWkldTplGenerator(o *packageSvcOpts) (workloadTemplateGenerator, error) {
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
	}
	targetEnv, err := o.getTargetEnv()
	if err != nil {
		return nil, err
	}
	var deployer workloadTemplateGenerator
	in := clideploy.WorkloadDeployerInput{
		SessionProvider: o.sessProvider,
		Name:            o.name,
		App:             targetApp,
		Env:             targetEnv,
		ImageTag:        o.tag,
		Mft:             o.appliedManifest,
	}
	switch t := o.appliedManifest.(type) {
	case *manifest.LoadBalancedWebService:
		deployer, err = clideploy.NewLBDeployer(&in)
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
	appTemplates, err := o.getSvcTemplates(targetEnv)
	if err != nil {
		return err
	}
	if _, err = o.stackWriter.Write([]byte(appTemplates.stack)); err != nil {
		return err
	}
	if _, err = o.paramsWriter.Write([]byte(appTemplates.configuration)); err != nil {
		return err
	}
	addonsTemplate, err := o.getAddonsTemplate()
	// return nil if addons not found.
	var notFoundErr *addon.ErrAddonsNotFound
	if errors.As(err, &notFoundErr) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("retrieve addons template: %w", err)
	}
	// Addons template won't show up without setting --output-dir flag.
	if o.outputDir != "" {
		if err := o.setAddonsFileWriter(); err != nil {
			return err
		}
	}
	_, err = o.addonsWriter.Write([]byte(addonsTemplate))
	return err
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

func (o *packageSvcOpts) getAddonsTemplate() (string, error) {
	if err := o.initAddonsClient(o); err != nil {
		return "", err
	}
	return o.addonsClient.Template()
}

func (o *packageSvcOpts) configureClients() error {
	o.tag = imageTagFromGit(o.runner, o.tag) // Best effort assign git tag.
	// client to retrieve an application's resources created with CloudFormation.
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	return nil
}

type wkldCfnTemplates struct {
	stack         string
	configuration string
}

// getSvcTemplates returns the CloudFormation stack's template and its parameters for the service.
func (o *packageSvcOpts) getSvcTemplates(env *config.Environment) (*wkldCfnTemplates, error) {
	mft, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		interpolator: o.newInterpolator(o.appName, o.envName),
		ws:           o.ws,
		unmarshal:    o.unmarshal,
		targetEnv:    o.targetEnv,
		svcType:      "",
	})
	if err != nil {
		return nil, err
	}
	o.appliedManifest = mft
	generator, err := o.newTplGenerator(o)
	if err != nil {
		return nil, err
	}
	uploadOut := clideploy.UploadArtifactsOutput{
		ImageDigest: aws.String(""),
	}
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
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
			RootUserARN: o.rootUserARN,
			Tags:        targetApp.Tags,
			ImageDigest: uploadOut.ImageDigest,
			EnvFileARN:  uploadOut.EnvFileARN,
			AddonsURL:   uploadOut.AddonsURL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate workload %s template against environment %s: %w", o.name, o.envName, err)
	}
	return &wkldCfnTemplates{stack: output.Template, configuration: output.Parameters}, nil
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
	o.stackWriter = templateFile

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

// RecommendActions is a no-op for this command.
func (o *packageSvcOpts) RecommendActions() error {
	return nil
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
		Short: "Prints the AWS CloudFormation template of a service.",
		Long:  `Prints the CloudFormation template used to deploy a service to an environment.`,
		Example: `
  Print the CloudFormation template for the "frontend" service parametrized for the "test" environment.
  /code $ copilot svc package -n frontend -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ copilot svc package -n frontend -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code frontend-test.stack.yml      frontend-test.params.yml`,
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
