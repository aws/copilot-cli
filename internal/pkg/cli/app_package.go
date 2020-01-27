// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
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

// PackageAppOpts holds the configuration needed to transform an application's manifest to CloudFormation.
type PackageAppOpts struct {
	// Fields with matching flags.
	AppName   string
	EnvName   string
	Tag       string
	OutputDir string

	// Interfaces to interact with dependencies.
	ws           archer.Workspace
	store        projectService
	describer    projectResourcesGetter
	stackWriter  io.Writer
	paramsWriter io.Writer
	fs           afero.Fs
	runner       runner

	*GlobalOpts // Embed global options.
}

// NewPackageAppOpts returns a new PackageAppOpts. The CloudFormation template is
// written to stdout and the parameters are discarded by default.
func NewPackageAppOpts() *PackageAppOpts {
	return &PackageAppOpts{
		runner:       command.New(),
		stackWriter:  os.Stdout,
		paramsWriter: ioutil.Discard,
		fs:           &afero.Afero{Fs: afero.NewOsFs()},
		GlobalOpts:   NewGlobalOpts(),
	}
}

// Validate returns an error if the values provided by the user are invalid.
func (o *PackageAppOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if o.AppName != "" {
		names, err := o.listAppNames()
		if err != nil {
			return err
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
func (o *PackageAppOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return o.askTag()
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *PackageAppOpts) Execute() error {
	env, err := o.store.GetEnvironment(o.ProjectName(), o.EnvName)
	if err != nil {
		return err
	}

	if o.OutputDir != "" {
		if err := o.setFileWriters(); err != nil {
			return err
		}
	}

	templates, err := o.getTemplates(env)
	if err != nil {
		return err
	}
	if _, err = o.stackWriter.Write([]byte(templates.stack)); err != nil {
		return err
	}
	_, err = o.paramsWriter.Write([]byte(templates.configuration))
	return err
}

func (o *PackageAppOpts) askAppName() error {
	if o.AppName != "" {
		return nil
	}

	names, err := o.listAppNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("there are no applications in the workspace, run `dw_run.sh init` first")
	}
	app, err := o.prompt.SelectOne(appPackageAppNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("prompt application name: %w", err)
	}
	o.AppName = app
	return nil
}

func (o *PackageAppOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	names, err := o.listEnvNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("there are no environments in project %s", o.ProjectName())
	}
	env, err := o.prompt.SelectOne(appPackageEnvNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("prompt environment name: %w", err)
	}
	o.EnvName = env
	return nil
}

func (o *PackageAppOpts) askTag() error {
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

func (o *PackageAppOpts) listAppNames() ([]string, error) {
	apps, err := o.ws.Apps()
	if err != nil {
		return nil, fmt.Errorf("list applications in workspace: %w", err)
	}
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.AppName())
	}
	return names, nil
}

type cfnTemplates struct {
	stack         string
	configuration string
}

// getTemplates returns the CloudFormation stack's template and its parameters.
func (o *PackageAppOpts) getTemplates(env *archer.Environment) (*cfnTemplates, error) {
	raw, err := o.ws.ReadFile(o.ws.AppManifestFileName(o.AppName))
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalApp(raw)
	if err != nil {
		return nil, err
	}

	proj, err := o.store.GetProject(o.ProjectName())
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

	switch t := mft.(type) {
	case *manifest.LBFargateManifest:
		createLBAppInput := &deploy.CreateLBFargateAppInput{
			App:          mft.(*manifest.LBFargateManifest),
			Env:          env,
			ImageRepoURL: repoURL,
			ImageTag:     o.Tag,
		}
		var appStack *stack.LBFargateStackConfig
		// If the project supports DNS Delegation, we'll also
		// make sure the app supports HTTPS
		if proj.RequiresDNSDelegation() {
			appStack = stack.NewHTTPSLBFargateStack(createLBAppInput)
		} else {
			appStack = stack.NewLBFargateStack(createLBAppInput)
		}

		tpl, err := appStack.Template()
		if err != nil {
			return nil, err
		}
		params, err := appStack.SerializedParameters()
		if err != nil {
			return nil, err
		}
		return &cfnTemplates{stack: tpl, configuration: params}, nil
	default:
		return nil, fmt.Errorf("create CloudFormation template for manifest of type %T", t)
	}
}

// setFileWriters creates the output directory, and updates the template and param writers to file writers in the directory.
func (o *PackageAppOpts) setFileWriters() error {
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

func contains(s string, items []string) bool {
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}

func (o *PackageAppOpts) listEnvNames() ([]string, error) {
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
	opts := NewPackageAppOpts()
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of an application.",
		Long:  `Prints the CloudFormation template used to deploy an application to an environment.`,
		Example: `
  Print the CloudFormation template for the "frontend" application parametrized for the "dev" environment.
  /code $ dw_run.sh app package -n frontend -e dev

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ dw_run.sh app package -n frontend -e dev --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code frontend.stack.yml      frontend-test.config.yml`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("new workspace: %w", err)
			}
			opts.ws = ws

			store, err := store.New()
			if err != nil {
				return fmt.Errorf("couldn't connect to application datastore: %w", err)
			}
			opts.store = store

			p := session.NewProvider()
			sess, err := p.Default()
			if err != nil {
				return fmt.Errorf("error retrieving default session: %w", err)
			}
			opts.describer = cloudformation.New(sess)
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().StringVarP(&opts.AppName, nameFlag, nameFlagShort, opts.AppName, appFlagDescription)
	cmd.Flags().StringVarP(&opts.EnvName, envFlag, envFlagShort, opts.EnvName, envFlagDescription)
	cmd.Flags().StringVar(&opts.Tag, imageTagFlag, opts.Tag, imageTagFlagDescription)
	cmd.Flags().StringVar(&opts.OutputDir, stackOutputDirFlag, opts.OutputDir, stackOutputDirFlagDescription)
	return cmd
}
