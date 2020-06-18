// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addon"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/selector"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	svcPackageSvcNamePrompt = "Which service would you like to generate a CloudFormation template for?"
	svcPackageEnvNamePrompt = "Which environment would you like to package this stack for?"
)

var initPackageAddonsSvc = func(o *packageSvcOpts) error {
	addonsSvc, err := addon.New(o.Name)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addonsSvc = addonsSvc
	return nil
}

type packageSvcVars struct {
	*GlobalOpts
	Name      string
	EnvName   string
	Tag       string
	OutputDir string
}

type packageSvcOpts struct {
	packageSvcVars

	// Interfaces to interact with dependencies.
	addonsSvc       templater
	initAddonsSvc   func(*packageSvcOpts) error // Overriden in tests.
	ws              wsSvcReader
	store           store
	appCFN          appResourcesGetter
	stackWriter     io.Writer
	paramsWriter    io.Writer
	addonsWriter    io.Writer
	fs              afero.Fs
	runner          runner
	sel             wsSelector
	stackSerializer func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error)
}

func newPackageSvcOpts(vars packageSvcVars) (*packageSvcOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	p := session.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, fmt.Errorf("retrieve default session: %w", err)
	}

	opts := &packageSvcOpts{
		packageSvcVars: vars,
		initAddonsSvc:  initPackageAddonsSvc,
		ws:             ws,
		store:          store,
		appCFN:         cloudformation.New(sess),
		runner:         command.New(),
		sel:            selector.NewWorkspaceSelect(vars.prompt, store, ws),
		stackWriter:    os.Stdout,
		paramsWriter:   ioutil.Discard,
		addonsWriter:   ioutil.Discard,
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
	}

	opts.stackSerializer = func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error) {
		var serializer stackSerializer
		switch v := mft.(type) {
		case *manifest.LoadBalancedWebService:
			if app.RequiresDNSDelegation() {
				serializer, err = stack.NewHTTPSLoadBalancedWebService(v, env.Name, app.Name, rc)
				if err != nil {
					return nil, fmt.Errorf("init https load balanced web service stack serializer: %w", err)
				}
			}
			serializer, err = stack.NewLoadBalancedWebService(v, env.Name, app.Name, rc)
			if err != nil {
				return nil, fmt.Errorf("init load balanced web service stack serializer: %w", err)
			}
		case *manifest.BackendService:
			serializer, err = stack.NewBackendService(v, env.Name, app.Name, rc)
			if err != nil {
				return nil, fmt.Errorf("init backend service stack serializer: %w", err)
			}
		default:
			return nil, fmt.Errorf("create stack serializer for manifest of type %T", v)
		}
		return serializer, nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageSvcOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.Name != "" {
		names, err := o.ws.ServiceNames()
		if err != nil {
			return fmt.Errorf("list services in the workspace: %w", err)
		}
		if !contains(o.Name, names) {
			return fmt.Errorf("service '%s' does not exist in the workspace", o.Name)
		}
	}
	if o.EnvName != "" {
		if _, err := o.store.GetEnvironment(o.AppName(), o.EnvName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any missing required fields.
func (o *packageSvcOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return o.askTag()
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageSvcOpts) Execute() error {
	env, err := o.store.GetEnvironment(o.AppName(), o.EnvName)
	if err != nil {
		return err
	}

	if o.OutputDir != "" {
		if err := o.setOutputFileWriters(); err != nil {
			return err
		}
	}

	appTemplates, err := o.getSvcTemplates(env)
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
	// return nil if addons dir doesn't exist.
	var notExistErr *addon.ErrDirNotExist
	if errors.As(err, &notExistErr) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("retrieve addons template: %w", err)
	}

	// Addons template won't show up without setting --output-dir flag.
	if o.OutputDir != "" {
		if err := o.setAddonsFileWriter(); err != nil {
			return err
		}
	}

	_, err = o.addonsWriter.Write([]byte(addonsTemplate))
	return err
}

func (o *packageSvcOpts) askAppName() error {
	if o.Name != "" {
		return nil
	}

	name, err := o.sel.Service(svcPackageSvcNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.Name = name
	return nil
}

func (o *packageSvcOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	name, err := o.sel.Environment(svcPackageEnvNamePrompt, "", o.AppName())
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.EnvName = name
	return nil
}

func (o *packageSvcOpts) askTag() error {
	if o.Tag != "" {
		return nil
	}

	tag, err := getVersionTag(o.runner)
	if err != nil {
		// We're not in a Git repository, prompt the user for an explicit tag.
		tag, err = o.prompt.Get(inputImageTagPrompt, "", nil)
		if err != nil {
			return fmt.Errorf("prompt get image tag: %w", err)
		}
	}
	o.Tag = tag
	return nil
}

func (o *packageSvcOpts) getAddonsTemplate() (string, error) {
	if err := o.initAddonsSvc(o); err != nil {
		return "", err
	}
	return o.addonsSvc.Template()
}

type svcCfnTemplates struct {
	stack         string
	configuration string
}

// getSvcTemplates returns the CloudFormation stack's template and its parameters for the service.
func (o *packageSvcOpts) getSvcTemplates(env *config.Environment) (*svcCfnTemplates, error) {
	raw, err := o.ws.ReadServiceManifest(o.Name)
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalService(raw)
	if err != nil {
		return nil, err
	}

	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		return nil, err
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(app, env.Region)
	if err != nil {
		return nil, err
	}

	repoURL, ok := resources.RepositoryURLs[o.Name]
	if !ok {
		return nil, &errRepoNotFound{
			svcName:      o.Name,
			envRegion:    env.Region,
			appAccountID: app.AccountID,
		}
	}
	serializer, err := o.stackSerializer(mft, env, app, stack.RuntimeConfig{
		ImageRepoURL:   repoURL,
		ImageTag:       o.Tag,
		AdditionalTags: app.Tags,
	})
	if err != nil {
		return nil, err
	}
	tpl, err := serializer.Template()
	if err != nil {
		return nil, fmt.Errorf("generate stack template: %w", err)
	}
	params, err := serializer.SerializedParameters()
	if err != nil {
		return nil, fmt.Errorf("generate stack template configuration: %w", err)
	}
	return &svcCfnTemplates{stack: tpl, configuration: params}, nil
}

// setOutputFileWriters creates the output directory, and updates the template and param writers to file writers in the directory.
func (o *packageSvcOpts) setOutputFileWriters() error {
	if err := o.fs.MkdirAll(o.OutputDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", o.OutputDir, err)
	}

	templatePath := filepath.Join(o.OutputDir,
		fmt.Sprintf(config.ServiceCfnTemplateNameFormat, o.Name))
	templateFile, err := o.fs.Create(templatePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", templatePath, err)
	}
	o.stackWriter = templateFile

	paramsPath := filepath.Join(o.OutputDir,
		fmt.Sprintf(config.ServiceCfnTemplateConfigurationNameFormat, o.Name, o.EnvName))
	paramsFile, err := o.fs.Create(paramsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", paramsPath, err)
	}
	o.paramsWriter = paramsFile

	return nil
}

func (o *packageSvcOpts) setAddonsFileWriter() error {
	addonsPath := filepath.Join(o.OutputDir,
		fmt.Sprintf(config.AddonsCfnTemplateNameFormat, o.Name))
	addonsFile, err := o.fs.Create(addonsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", addonsPath, err)
	}
	o.addonsWriter = addonsFile

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

type errRepoNotFound struct {
	svcName      string
	envRegion    string
	appAccountID string
}

func (e *errRepoNotFound) Error() string {
	return fmt.Sprintf("ECR repository not found for service %s in region %s and account %s", e.svcName, e.envRegion, e.appAccountID)
}

func (e *errRepoNotFound) Is(target error) bool {
	t, ok := target.(*errRepoNotFound)
	if !ok {
		return false
	}
	return e.svcName == t.svcName &&
		e.envRegion == t.envRegion &&
		e.appAccountID == t.appAccountID
}

// BuildSvcPackageCmd builds the command for printing a service's CloudFormation template.
func BuildSvcPackageCmd() *cobra.Command {
	vars := packageSvcVars{
		GlobalOpts: NewGlobalOpts(),
	}
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
  /code frontend.stack.yml      frontend-test.config.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageSvcOpts(vars)
			if err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// Set the defaults to opts.{Field} otherwise cobra overrides the values set by the constructor.
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.Tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.OutputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}
