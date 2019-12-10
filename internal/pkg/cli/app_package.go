// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
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

// NewPackageAppOpts returns a new PackageAppOpts where the image tag is set to "manual-{short git sha}".
// The CloudFormation template is written to stdout and the parameters are discarded by default.
// If an error occurred while running git, we leave the image tag empty "".
func NewPackageAppOpts() *PackageAppOpts {
	opts := &PackageAppOpts{
		runner:       command.New(),
		stackWriter:  os.Stdout,
		paramsWriter: ioutil.Discard,
		fs:           &afero.Afero{Fs: afero.NewOsFs()},
		GlobalOpts:   NewGlobalOpts(),
	}

	var buf bytes.Buffer
	if err := opts.runner.Run("git", []string{"rev-parse", "--short", "HEAD"}, command.Stdout(&buf)); err != nil {
		return opts
	}

	opts.Tag = fmt.Sprintf("manual-%s", strings.TrimSpace(buf.String()))

	return opts
}

// Ask prompts the user for any missing required fields.
func (opts *PackageAppOpts) Ask() error {
	if opts.AppName == "" {
		names, err := opts.listAppNames()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return errors.New("there are no applications in the workspace, run `ecs-preview init` first")
		}
		app, err := opts.prompt.SelectOne(appPackageAppNamePrompt, "", names)
		if err != nil {
			return fmt.Errorf("prompt application name: %w", err)
		}
		opts.AppName = app
	}
	if opts.EnvName == "" {
		names, err := opts.listEnvNames()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("there are no environments in project %s", opts.ProjectName())
		}
		env, err := opts.prompt.SelectOne(appPackageEnvNamePrompt, "", names)
		if err != nil {
			return fmt.Errorf("prompt environment name: %w", err)
		}
		opts.EnvName = env
	}
	return nil
}

// Validate returns an error if the values provided by the user are invalid.
func (opts *PackageAppOpts) Validate() error {
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if opts.Tag == "" {
		return fmt.Errorf("image tag cannot be empty, please provide the %s flag", color.HighlightCode("--tag"))
	}
	if opts.AppName != "" {
		names, err := opts.listAppNames()
		if err != nil {
			return err
		}
		if !contains(opts.AppName, names) {
			return fmt.Errorf("application '%s' does not exist in the workspace", opts.AppName)
		}
	}
	if opts.EnvName != "" {
		if _, err := opts.store.GetEnvironment(opts.ProjectName(), opts.EnvName); err != nil {
			return err
		}
	}
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (opts *PackageAppOpts) Execute() error {
	env, err := opts.store.GetEnvironment(opts.ProjectName(), opts.EnvName)
	if err != nil {
		return err
	}

	if opts.OutputDir != "" {
		if err := opts.setFileWriters(); err != nil {
			return err
		}
	}

	templates, err := opts.getTemplates(env)
	if err != nil {
		return err
	}
	if _, err = opts.stackWriter.Write([]byte(templates.stack)); err != nil {
		return err
	}
	_, err = opts.paramsWriter.Write([]byte(templates.configuration))
	return err
}

func (opts *PackageAppOpts) listAppNames() ([]string, error) {
	apps, err := opts.ws.Apps()
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
func (opts *PackageAppOpts) getTemplates(env *archer.Environment) (*cfnTemplates, error) {
	raw, err := opts.ws.ReadFile(opts.ws.AppManifestFileName(opts.AppName))
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalApp(raw)
	if err != nil {
		return nil, err
	}

	proj, err := opts.store.GetProject(opts.ProjectName())
	if err != nil {
		return nil, err
	}
	resources, err := opts.describer.GetProjectResourcesByRegion(proj, env.Region)
	if err != nil {
		return nil, err
	}

	repoURL, ok := resources.RepositoryURLs[opts.AppName]
	if !ok {
		return nil, &errRepoNotFound{
			appName:       opts.AppName,
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
			ImageTag:     opts.Tag,
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
func (opts *PackageAppOpts) setFileWriters() error {
	if err := opts.fs.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", opts.OutputDir, err)
	}

	templatePath := filepath.Join(opts.OutputDir,
		fmt.Sprintf(archer.AppCfnTemplateNameFormat, opts.AppName))
	templateFile, err := opts.fs.Create(templatePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", templatePath, err)
	}
	opts.stackWriter = templateFile

	paramsPath := filepath.Join(opts.OutputDir,
		fmt.Sprintf(archer.AppCfnTemplateConfigurationNameFormat, opts.AppName, opts.EnvName))
	paramsFile, err := opts.fs.Create(paramsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", paramsPath, err)
	}
	opts.paramsWriter = paramsFile
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

func (opts *PackageAppOpts) listEnvNames() ([]string, error) {
	envs, err := opts.store.ListEnvironments(opts.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", opts.ProjectName(), err)
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
  Print the CloudFormation template for the "frontend" application parametrized for the "test" environment.
  /code $ ecs-preview app package -n frontend -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ ecs-preview app package -n frontend -e test --output-dir ./infrastructure
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

			sess, err := session.Default()
			if err != nil {
				return fmt.Errorf("error retrieving default session: %w", err)
			}
			opts.describer = cloudformation.New(sess)
			return opts.Validate()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
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
