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

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	appPackageAppNamePrompt = "Which application would you like to generate a CloudFormation template for?"
	appPackageEnvNamePrompt = "Which environment would you like to create this stack for?"
)

var initPackageAddonsSvc = func(o *packageAppOpts) error {
	addonsSvc, err := addons.New(o.AppName)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addonsSvc = addonsSvc

	return nil
}

type packageAppVars struct {
	*GlobalOpts
	AppName   string
	EnvName   string
	Tag       string
	OutputDir string
}

type packageAppOpts struct {
	packageAppVars

	// Interfaces to interact with dependencies.
	addonsSvc       templater
	initAddonsSvc   func(*packageAppOpts) error // Overriden in tests.
	ws              wsAppReader
	store           projectService
	describer       projectResourcesGetter
	stackWriter     io.Writer
	paramsWriter    io.Writer
	addonsWriter    io.Writer
	fs              afero.Fs
	runner          runner
	stackSerializer func(mft interface{}, env *archer.Environment, proj *archer.Project, rc stack.RuntimeConfig) (stackSerializer, error)
}

func newPackageAppOpts(vars packageAppVars) (*packageAppOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to application datastore: %w", err)
	}
	p := session.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, fmt.Errorf("error retrieving default session: %w", err)
	}

	opts := &packageAppOpts{
		packageAppVars: vars,
		initAddonsSvc:  initPackageAddonsSvc,
		ws:             ws,
		store:          store,
		describer:      cloudformation.New(sess),
		runner:         command.New(),
		stackWriter:    os.Stdout,
		paramsWriter:   ioutil.Discard,
		addonsWriter:   ioutil.Discard,
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
	}

	opts.stackSerializer = func(mft interface{}, env *archer.Environment, proj *archer.Project, rc stack.RuntimeConfig) (stackSerializer, error) {
		var serializer stackSerializer
		switch v := mft.(type) {
		case *manifest.LoadBalancedWebApp:
			if proj.RequiresDNSDelegation() {
				serializer, err = stack.NewHTTPSLoadBalancedWebApp(v, env.Name, proj.Name, rc)
				if err != nil {
					return nil, fmt.Errorf("init https load balanced web app stack serializer: %w", err)
				}
			}
			serializer, err = stack.NewLoadBalancedWebApp(v, env.Name, proj.Name, rc)
			if err != nil {
				return nil, fmt.Errorf("init load balanced web app stack serializer: %w", err)
			}
		case *manifest.BackendApp:
			serializer, err = stack.NewBackendApp(v, env.Name, proj.Name, rc)
			if err != nil {
				return nil, fmt.Errorf("init backend app stack serializer: %w", err)
			}
		default:
			return nil, fmt.Errorf("create stack serializer for manifest of type %T", v)
		}
		return serializer, nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageAppOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if o.AppName != "" {
		names, err := o.ws.AppNames()
		if err != nil {
			return fmt.Errorf("list applications in workspace: %w", err)
		}
		if !contains(o.AppName, names) {
			return fmt.Errorf("application '%s' does not exist in the workspace", o.AppName)
		}
	}
	if o.EnvName != "" {
		if _, err := o.store.GetEnvironment(o.ProjectName(), o.EnvName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any missing required fields.
func (o *packageAppOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return o.askTag()
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageAppOpts) Execute() error {
	env, err := o.store.GetEnvironment(o.ProjectName(), o.EnvName)
	if err != nil {
		return err
	}

	if o.OutputDir != "" {
		if err := o.setAppFileWriters(); err != nil {
			return err
		}
	}

	appTemplates, err := o.getAppTemplates(env)
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
	var notExistErr *addons.ErrDirNotExist
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

func (o *packageAppOpts) askAppName() error {
	if o.AppName != "" {
		return nil
	}

	appNames, err := o.ws.AppNames()
	if err != nil {
		return fmt.Errorf("list applications in workspace: %w", err)
	}
	if len(appNames) == 0 {
		return errors.New("there are no applications in the workspace, run `ecs-preview init` first")
	}
	if len(appNames) == 1 {
		o.AppName = appNames[0]
		return nil
	}
	appName, err := o.prompt.SelectOne(appPackageAppNamePrompt, "", appNames)
	if err != nil {
		return fmt.Errorf("prompt application name: %w", err)
	}
	o.AppName = appName
	return nil
}

func (o *packageAppOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	envNames, err := o.listEnvNames()
	if err != nil {
		return err
	}
	if len(envNames) == 0 {
		return fmt.Errorf("there are no environments in project %s", o.ProjectName())
	}
	if len(envNames) == 1 {
		o.EnvName = envNames[0]
		return nil
	}
	envName, err := o.prompt.SelectOne(appPackageEnvNamePrompt, "", envNames)
	if err != nil {
		return fmt.Errorf("prompt environment name: %w", err)
	}
	o.EnvName = envName
	return nil
}

func (o *packageAppOpts) askTag() error {
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

func (o *packageAppOpts) getAddonsTemplate() (string, error) {
	if err := o.initAddonsSvc(o); err != nil {
		return "", err
	}
	return o.addonsSvc.Template()
}

type appCfnTemplates struct {
	stack         string
	configuration string
}

// getAppTemplates returns the CloudFormation stack's template and its parameters for the application.
func (o *packageAppOpts) getAppTemplates(env *archer.Environment) (*appCfnTemplates, error) {
	raw, err := o.ws.ReadAppManifest(o.AppName)
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalApp(raw)
	if err != nil {
		return nil, err
	}

	proj, err := o.store.GetApplication(o.ProjectName())
	if err != nil {
		return nil, err
	}
	resources, err := o.describer.GetProjectResourcesByRegion(proj, env.Region)
	if err != nil {
		return nil, err
	}

	repoURL, ok := resources.RepositoryURLs[o.AppName]
	if !ok {
		return nil, &errRepoNotFound{
			appName:       o.AppName,
			envRegion:     env.Region,
			projAccountID: proj.AccountID,
		}
	}
	serializer, err := o.stackSerializer(mft, env, proj, stack.RuntimeConfig{
		ImageRepoURL:   repoURL,
		ImageTag:       o.Tag,
		AdditionalTags: proj.Tags,
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
	return &appCfnTemplates{stack: tpl, configuration: params}, nil
}

// setAppFileWriters creates the output directory, and updates the template and param writers to file writers in the directory.
func (o *packageAppOpts) setAppFileWriters() error {
	if err := o.fs.MkdirAll(o.OutputDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", o.OutputDir, err)
	}

	templatePath := filepath.Join(o.OutputDir,
		fmt.Sprintf(archer.AppCfnTemplateNameFormat, o.AppName))
	templateFile, err := o.fs.Create(templatePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", templatePath, err)
	}
	o.stackWriter = templateFile

	paramsPath := filepath.Join(o.OutputDir,
		fmt.Sprintf(archer.AppCfnTemplateConfigurationNameFormat, o.AppName, o.EnvName))
	paramsFile, err := o.fs.Create(paramsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", paramsPath, err)
	}
	o.paramsWriter = paramsFile

	return nil
}

func (o *packageAppOpts) setAddonsFileWriter() error {
	addonsPath := filepath.Join(o.OutputDir,
		fmt.Sprintf(archer.AddonsCfnTemplateNameFormat, o.AppName))
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

func (o *packageAppOpts) listEnvNames() ([]string, error) {
	envs, err := o.store.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", o.ProjectName(), err)
	}
	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}
	return names, nil
}

type errRepoNotFound struct {
	appName       string
	envRegion     string
	projAccountID string
}

func (e *errRepoNotFound) Error() string {
	return fmt.Sprintf("ECR repository not found for application %s in region %s and account %s", e.appName, e.envRegion, e.projAccountID)
}

func (e *errRepoNotFound) Is(target error) bool {
	t, ok := target.(*errRepoNotFound)
	if !ok {
		return false
	}
	return e.appName == t.appName &&
		e.envRegion == t.envRegion &&
		e.projAccountID == t.projAccountID
}

// BuildAppPackageCmd builds the command for printing an application's CloudFormation template.
func BuildAppPackageCmd() *cobra.Command {
	vars := packageAppVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of an application.",
		Long:  `Prints the CloudFormation template used to deploy an application to an environment.`,
		Example: `
  Print the CloudFormation template for the "frontend" application parametrized for the "test" environment.
  /code $ ecs-preview app package -n frontend -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ ecs-preview app package -n frontend -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code frontend.stack.yml      frontend-test.config.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageAppOpts(vars)
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
	cmd.Flags().StringVarP(&vars.AppName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.Tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.OutputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}
