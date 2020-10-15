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

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	jobPackageJobNamePrompt = "Which job would you like to generate a CloudFormation template for?"
	jobPackageEnvNamePrompt = "Which environment would you like to package this stack for?"
)

var initPackageAddons = func(o *packageJobOpts) error {
	addonsSvc, err := addon.New(o.name, "job")
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addonsSvc = addonsSvc
	return nil
}

type packageJobVars struct {
	name      string
	envName   string
	appName   string
	tag       string
	outputDir string
}

type packageJobOpts struct {
	packageJobVars

	// Interfaces to interact with dependencies.
	addonsSvc       templater
	initAddonsSvc   func(*packageJobOpts) error // Overridden in tests.
	ws              wsJobDirReader
	store           store
	appCFN          appResourcesGetter
	stackWriter     io.Writer
	paramsWriter    io.Writer
	addonsWriter    io.Writer
	fs              afero.Fs
	runner          runner
	sel             wsSelector
	prompt          prompter
	stackSerializer func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error)
}

func newPackageJobOpts(vars packageJobVars) (*packageJobOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, fmt.Errorf("retrieve default session: %w", err)
	}

	prompter := prompt.New()
	sel, err := selector.NewWorkspaceSelect(prompter, store, ws)
	if err != nil {
		return nil, err
	}
	opts := &packageJobOpts{
		packageJobVars: vars,
		initAddonsSvc:  initPackageAddons,
		ws:             ws,
		store:          store,
		appCFN:         cloudformation.New(sess),
		runner:         command.New(),
		sel:            sel,
		prompt:         prompter,
		stackWriter:    os.Stdout,
		paramsWriter:   ioutil.Discard,
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
	}

	opts.stackSerializer = func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error) {
		var serializer stackSerializer
		jobMft := mft.(*manifest.ScheduledJob)
		serializer, err := stack.NewScheduledJob(jobMft, env.Name, app.Name, rc)
		if err != nil {
			return nil, fmt.Errorf("init scheduled job stack serializer: %w", err)
		}
		return serializer, nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		names, err := o.ws.JobNames()
		if err != nil {
			return fmt.Errorf("list jobs in the workspace: %w", err)
		}
		if !contains(o.name, names) {
			return fmt.Errorf("job '%s' does not exist in the workspace", o.name)
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any missing required fields.
func (o *packageJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	tag, err := askImageTag(o.tag, o.prompt, o.runner)
	if err != nil {
		return err
	}
	o.tag = tag
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageJobOpts) Execute() error {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return err
	}

	if o.outputDir != "" {
		if err := o.setOutputFileWriters(); err != nil {
			return err
		}
	}
	jobTemplates, err := o.getJobTemplates(env)
	if err != nil {
		return err
	}
	if _, err = o.stackWriter.Write([]byte(jobTemplates.stack)); err != nil {
		return err
	}
	if _, err = o.paramsWriter.Write([]byte(jobTemplates.configuration)); err != nil {
		return err
	}

	addonsTemplate, err := o.getAddonsTemplate()
	// Return nil if addons dir doesn't exist.
	var notExistErr *addon.ErrAddonsDirNotExist
	if errors.As(err, &notExistErr) {
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

func (o *packageJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job(jobPackageJobNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *packageJobOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment(jobPackageEnvNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

// setOutputFileWriters creates the output directory, and updates the template and param writers to file writers in the directory.
func (o *packageJobOpts) setOutputFileWriters() error {
	if err := o.fs.MkdirAll(o.outputDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", o.outputDir, err)
	}

	templatePath := filepath.Join(o.outputDir,
		fmt.Sprintf(config.WorkloadCfnTemplateNameFormat, o.name))
	templateFile, err := o.fs.Create(templatePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", templatePath, err)
	}
	o.stackWriter = templateFile

	paramsPath := filepath.Join(o.outputDir,
		fmt.Sprintf(config.WorkloadCfnTemplateConfigurationNameFormat, o.name, o.envName))
	paramsFile, err := o.fs.Create(paramsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", paramsPath, err)
	}
	o.paramsWriter = paramsFile

	return nil
}

type jobCfnTemplates struct {
	stack         string
	configuration string
}

// getJobTemplates returns the CloudFormation stack's template and its parameters for the job.
func (o *packageJobOpts) getJobTemplates(env *config.Environment) (*jobCfnTemplates, error) {
	raw, err := o.ws.ReadJobManifest(o.name)
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalWorkload(raw)
	if err != nil {
		return nil, err
	}
	imgNeedsBuild, err := manifest.JobDockerfileBuildRequired(mft)
	if err != nil {
		return nil, err
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, err
	}
	rc := stack.RuntimeConfig{
		AdditionalTags: app.Tags,
	}
	if imgNeedsBuild {
		resources, err := o.appCFN.GetAppResourcesByRegion(app, env.Region)
		if err != nil {
			return nil, err
		}
		repoURL, ok := resources.RepositoryURLs[o.name]
		if !ok {
			return nil, &errRepoNotFound{
				wlName:       o.name,
				envRegion:    env.Region,
				appAccountID: app.AccountID,
			}
		}
		rc.Image = &stack.ECRImage{
			RepoURL:  repoURL,
			ImageTag: o.tag,
		}
	}
	serializer, err := o.stackSerializer(mft, env, app, rc)
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
	return &jobCfnTemplates{stack: tpl, configuration: params}, nil
}

func (o *packageJobOpts) getAddonsTemplate() (string, error) {
	if err := o.initAddonsSvc(o); err != nil {
		return "", err
	}
	return o.addonsSvc.Template()
}

func (o *packageJobOpts) setAddonsFileWriter() error {
	addonsPath := filepath.Join(o.outputDir,
		fmt.Sprintf(config.AddonsCfnTemplateNameFormat, o.name))
	addonsFile, err := o.fs.Create(addonsPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", addonsPath, err)
	}
	o.addonsWriter = addonsFile

	return nil
}

// buildJobPackageCmd builds the command for printing a job's CloudFormation template.
func buildJobPackageCmd() *cobra.Command {
	vars := packageJobVars{}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of a job.",
		Long:  `Prints the CloudFormation template used to deploy a job to an environment.`,
		Example: `
  Print the CloudFormation template for the "report-generator" job parametrized for the "test" environment.
  /code $ copilot job package -n report-generator -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ copilot job package -n report-generator -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code report-generator.stack.yml      report-generator-test.config.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageJobOpts(vars)
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
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}
