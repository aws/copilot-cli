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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
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
	name      string
	envName   string
	appName   string
	tag       string
	outputDir string
}

type packageSvcOpts struct {
	packageSvcVars

	// Interfaces to interact with dependencies.
	addonsClient      templater
	initAddonsClient  func(*packageSvcOpts) error // Overridden in tests.
	ws                wsSvcReader
	store             store
	appCFN            appResourcesGetter
	stackWriter       io.Writer
	paramsWriter      io.Writer
	addonsWriter      io.Writer
	fs                afero.Fs
	runner            runner
	sel               wsSelector
	prompt            prompter
	stackSerializer   func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error)
	newEndpointGetter func(app, env string) (endpointGetter, error)
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
	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, fmt.Errorf("retrieve default session: %w", err)
	}
	prompter := prompt.New()
	opts := &packageSvcOpts{
		packageSvcVars:   vars,
		initAddonsClient: initPackageAddonsClient,
		ws:               ws,
		store:            store,
		appCFN:           cloudformation.New(sess),
		runner:           exec.NewCmd(),
		sel:              selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:           prompter,
		stackWriter:      os.Stdout,
		paramsWriter:     ioutil.Discard,
		addonsWriter:     ioutil.Discard,
		fs:               &afero.Afero{Fs: afero.NewOsFs()},
	}
	appVersionGetter, err := describe.NewAppDescriber(vars.appName)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", vars.name, err)
	}
	opts.stackSerializer = func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error) {
		var serializer stackSerializer
		switch t := mft.(type) {
		case *manifest.LoadBalancedWebService:
			if app.RequiresDNSDelegation() {
				if err := validateAlias(aws.StringValue(t.Name), aws.StringValue(t.Alias), app, env.Name, appVersionGetter); err != nil {
					return nil, err
				}
				serializer, err = stack.NewHTTPSLoadBalancedWebService(t, env.Name, app.Name, rc)
				if err != nil {
					return nil, fmt.Errorf("init https load balanced web service stack serializer: %w", err)
				}
			} else {
				serializer, err = stack.NewLoadBalancedWebService(t, env.Name, app.Name, rc)
				if err != nil {
					return nil, fmt.Errorf("init load balanced web service stack serializer: %w", err)
				}
			}
		case *manifest.RequestDrivenWebService:
			appInfo := deploy.AppInformation{
				Name:                env.App,
				DNSName:             app.Domain,
				AccountPrincipalARN: app.AccountID,
			}
			serializer, err = stack.NewRequestDrivenWebService(t, env.Name, appInfo, rc)
			if err != nil {
				return nil, fmt.Errorf("init request-driven web service stack serializer: %w", err)
			}
		case *manifest.BackendService:
			serializer, err = stack.NewBackendService(t, env.Name, app.Name, rc)
			if err != nil {
				return nil, fmt.Errorf("init backend service stack serializer: %w", err)
			}
		default:
			return nil, fmt.Errorf("create stack serializer for manifest of type %T", t)
		}
		return serializer, nil
	}
	opts.newEndpointGetter = func(app, env string) (endpointGetter, error) {
		d, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         app,
			Env:         env,
			ConfigStore: store,
		})
		if err != nil {
			return nil, fmt.Errorf("new env describer for environment %s in app %s: %v", env, app, err)
		}
		return d, nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageSvcOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		names, err := o.ws.ServiceNames()
		if err != nil {
			return fmt.Errorf("list services in the workspace: %w", err)
		}
		if !contains(o.name, names) {
			return fmt.Errorf("service '%s' does not exist in the workspace", o.name)
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
func (o *packageSvcOpts) Ask() error {
	if err := o.askSvcName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageSvcOpts) Execute() error {
	o.tag = imageTagFromGit(o.runner, o.tag) // Best effort assign git tag.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return err
	}

	if o.outputDir != "" {
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

func (o *packageSvcOpts) askSvcName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Service(svcPackageSvcNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *packageSvcOpts) askEnvName() error {
	if o.envName != "" {
		return nil
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

type svcCfnTemplates struct {
	stack         string
	configuration string
}

// getSvcTemplates returns the CloudFormation stack's template and its parameters for the service.
func (o *packageSvcOpts) getSvcTemplates(env *config.Environment) (*svcCfnTemplates, error) {
	raw, err := o.ws.ReadServiceManifest(o.name)
	if err != nil {
		return nil, err
	}
	mft, err := manifest.UnmarshalWorkload(raw)
	if err != nil {
		return nil, err
	}
	envMft, err := mft.ApplyEnv(o.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", o.envName, err)
	}
	imgNeedsBuild, err := manifest.ServiceDockerfileBuildRequired(envMft)
	if err != nil {
		return nil, err
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, err
	}
	endpointGetter, err := o.newEndpointGetter(o.appName, o.envName)
	if err != nil {
		return nil, err
	}
	endpoint, err := endpointGetter.ServiceDiscoveryEndpoint()
	if err != nil {
		return nil, err
	}
	rc := stack.RuntimeConfig{
		AdditionalTags:           app.Tags,
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                app.AccountID,
		Region:                   env.Region,
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
	serializer, err := o.stackSerializer(envMft, env, app, rc)
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

// RecommendedActions is a no-op for this command.
func (o *packageSvcOpts) RecommendedActions() []string {
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
	wlName       string
	envRegion    string
	appAccountID string
}

func (e *errRepoNotFound) Error() string {
	return fmt.Sprintf("ECR repository not found for service %s in region %s and account %s", e.wlName, e.envRegion, e.appAccountID)
}

func (e *errRepoNotFound) Is(target error) bool {
	t, ok := target.(*errRepoNotFound)
	if !ok {
		return false
	}
	return e.wlName == t.wlName &&
		e.envRegion == t.envRegion &&
		e.appAccountID == t.appAccountID
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

			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}
