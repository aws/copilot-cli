// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	addon "github.com/aws/copilot-cli/internal/pkg/addon"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	inputImageTagPrompt = "Input an image tag value:"
)

var (
	errNoLocalManifestsFound = errors.New("no manifest files found")
)

type deploySvcVars struct {
	*GlobalOpts
	Name         string
	EnvName      string
	ImageTag     string
	ResourceTags map[string]string
}

type deploySvcOpts struct {
	deploySvcVars

	store        store
	ws           wsSvcReader
	ecr          ecrService
	docker       dockerService
	s3           artifactUploader
	cmd          runner
	addons       templater
	appCFN       appResourcesGetter
	svcCFN       cloudformation.CloudFormation
	sessProvider sessionProvider

	spinner progress
	sel     wsSelector

	// cached variables
	targetApp         *config.Application
	targetEnvironment *config.Environment
	targetSvc         *config.Service
}

func newSvcDeployOpts(vars deploySvcVars) (*deploySvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	return &deploySvcOpts{
		deploySvcVars: vars,

		store:        store,
		ws:           ws,
		spinner:      termprogress.NewSpinner(),
		sel:          selector.NewWorkspaceSelect(vars.prompt, store, ws),
		docker:       docker.New(),
		cmd:          command.New(),
		sessProvider: session.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deploySvcOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.Name != "" {
		if err := o.validateSvcName(); err != nil {
			return err
		}
	}
	if o.EnvName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deploySvcOpts) Ask() error {
	if err := o.askSvcName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	if err := o.askImageTag(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the service,
func (o *deploySvcOpts) Execute() error {
	env, err := o.targetEnv()
	if err != nil {
		return err
	}
	o.targetEnvironment = env

	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		return err
	}
	o.targetApp = app

	svc, err := o.store.GetService(o.AppName(), o.Name)
	if err != nil {
		return fmt.Errorf("get service configuration: %w", err)
	}
	o.targetSvc = svc

	if err := o.configureClients(); err != nil {
		return err
	}

	if err := o.pushToECRRepo(); err != nil {
		return err
	}

	// TODO: delete addons template from S3 bucket when deleting the environment.
	addonsURL, err := o.pushAddonsTemplateToS3Bucket()
	if err != nil {
		return err
	}

	if err := o.deploySvc(addonsURL); err != nil {
		return err
	}

	return o.showAppURI()
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendedActions() []string {
	return nil
}

func (o *deploySvcOpts) validateSvcName() error {
	names, err := o.ws.ServiceNames()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %w", err)
	}
	for _, name := range names {
		if o.Name == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", color.HighlightUserInput(o.Name))
}

func (o *deploySvcOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deploySvcOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.AppName(), o.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s configuration: %w", o.EnvName, err)
	}
	return env, nil
}

func (o *deploySvcOpts) askSvcName() error {
	if o.Name != "" {
		return nil
	}

	name, err := o.sel.Service("Select a service in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.Name = name
	return nil
}

func (o *deploySvcOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	name, err := o.sel.Environment("Select an environment", "", o.AppName())
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.EnvName = name
	return nil
}

func (o *deploySvcOpts) askImageTag() error {
	if o.ImageTag != "" {
		return nil
	}

	tag, err := getVersionTag(o.cmd)

	if err == nil {
		o.ImageTag = tag

		return nil
	}

	log.Warningln("Failed to default tag, are you in a git repository?")

	userInputTag, err := o.prompt.Get(inputImageTagPrompt, "", nil /*no validation*/)
	if err != nil {
		return fmt.Errorf("prompt for image tag: %w", err)
	}
	o.ImageTag = userInputTag
	return nil
}

func (o *deploySvcOpts) configureClients() error {
	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", o.targetEnvironment.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(o.targetEnvironment.ManagerRoleARN, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("assuming environment manager role: %w", err)
	}

	// ECR client against tools account profile AND target environment region
	o.ecr = ecr.New(defaultSessEnvRegion)

	o.s3 = s3.New(defaultSessEnvRegion)

	// CF client against env account profile AND target environment region
	o.svcCFN = cloudformation.New(envSession)

	addonsSvc, err := addon.New(o.Name)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addons = addonsSvc

	// client to retrieve an application's resources created with CloudFormation
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	o.appCFN = cloudformation.New(defaultSess)
	return nil
}

func (o *deploySvcOpts) pushToECRRepo() error {
	repoName := fmt.Sprintf("%s/%s", o.appName, o.Name)

	uri, err := o.ecr.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("get ECR repository URI: %w", err)
	}

	path, err := o.getDockerfilePath()
	if err != nil {
		return err
	}

	if err := o.docker.Build(uri, o.ImageTag, path); err != nil {
		return fmt.Errorf("build Dockerfile at %s with tag %s: %w", path, o.ImageTag, err)
	}

	auth, err := o.ecr.GetECRAuth()

	if err != nil {
		return fmt.Errorf("get ECR auth data: %w", err)
	}

	o.docker.Login(uri, auth.Username, auth.Password)

	return o.docker.Push(uri, o.ImageTag)
}

func (o *deploySvcOpts) getDockerfilePath() (string, error) {
	type dfPath interface {
		DockerfilePath() string
	}

	manifestBytes, err := o.ws.ReadServiceManifest(o.Name)
	if err != nil {
		return "", fmt.Errorf("read manifest file %s: %w", o.Name, err)
	}

	svc, err := manifest.UnmarshalService(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("unmarshal svc manifest: %w", err)
	}

	mf, ok := svc.(dfPath)
	if !ok {
		return "", fmt.Errorf("service %s does not have a dockerfile path", o.Name)
	}
	return mf.DockerfilePath(), nil
}

// pushAddonsTemplateToS3Bucket generates the addons template for the service and pushes it to S3.
// If the service doesn't have any addons, it returns the empty string and no errors.
// If the service has addons, it returns the URL of the S3 object storing the addons template.
func (o *deploySvcOpts) pushAddonsTemplateToS3Bucket() (string, error) {
	template, err := o.addons.Template()
	if err != nil {
		var notExistErr *addon.ErrDirNotExist
		if errors.As(err, &notExistErr) {
			// addons doesn't exist for service, the url is empty.
			return "", nil
		}
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return "", fmt.Errorf("get app resources: %w", err)
	}

	reader := strings.NewReader(template)
	url, err := o.s3.PutArtifact(resources.S3Bucket, fmt.Sprintf(config.AddonsCfnTemplateNameFormat, o.Name), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", resources.S3Bucket, err)
	}
	return url, nil
}

func (o *deploySvcOpts) manifest() (interface{}, error) {
	raw, err := o.ws.ReadServiceManifest(o.Name)
	if err != nil {
		return nil, fmt.Errorf("read service %s manifest from workspace: %w", o.Name, err)
	}
	mft, err := manifest.UnmarshalService(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", o.Name, err)
	}
	return mft, nil
}

func (o *deploySvcOpts) runtimeConfig(addonsURL string) (*stack.RuntimeConfig, error) {
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return nil, fmt.Errorf("get application %s resources from region %s: %w", o.targetApp.Name, o.targetEnvironment.Region, err)
	}
	repoURL, ok := resources.RepositoryURLs[o.Name]
	if !ok {
		return nil, &errRepoNotFound{
			svcName:      o.Name,
			envRegion:    o.targetEnvironment.Region,
			appAccountID: o.targetApp.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		ImageRepoURL:      repoURL,
		ImageTag:          o.ImageTag,
		AddonsTemplateURL: addonsURL,
		AdditionalTags:    tags.Merge(o.targetApp.Tags, o.ResourceTags),
	}, nil
}

func (o *deploySvcOpts) stackConfiguration(addonsURL string) (cloudformation.StackConfiguration, error) {
	mft, err := o.manifest()
	if err != nil {
		return nil, err
	}
	rc, err := o.runtimeConfig(addonsURL)
	if err != nil {
		return nil, err
	}
	var conf cloudformation.StackConfiguration
	switch t := mft.(type) {
	case *manifest.LoadBalancedWebService:
		if o.targetApp.RequiresDNSDelegation() {
			conf, err = stack.NewHTTPSLoadBalancedWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
		} else {
			conf, err = stack.NewLoadBalancedWebService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
		}
	case *manifest.BackendService:
		conf, err = stack.NewBackendService(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

func (o *deploySvcOpts) deploySvc(addonsURL string) error {
	conf, err := o.stackConfiguration(addonsURL)
	if err != nil {
		return err
	}
	o.spinner.Start(
		fmt.Sprintf("Deploying %s to %s.",
			fmt.Sprintf("%s:%s", color.HighlightUserInput(o.Name), color.HighlightUserInput(o.ImageTag)),
			color.HighlightUserInput(o.targetEnvironment.Name)))

	if err := o.svcCFN.DeployService(conf, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		o.spinner.Stop(log.Serrorf("Failed to deploy service.\n"))
		return fmt.Errorf("deploy service: %w", err)
	}
	o.spinner.Stop("\n")
	return nil
}

func (o *deploySvcOpts) showAppURI() error {
	type identifier interface {
		URI(string) (string, error)
	}

	var svcDescriber identifier
	var err error
	switch o.targetSvc.Type {
	case manifest.LoadBalancedWebServiceType:
		svcDescriber, err = describe.NewWebServiceDescriber(o.AppName(), o.Name)
	case manifest.BackendServiceType:
		svcDescriber, err = describe.NewBackendServiceDescriber(o.AppName(), o.Name)
	default:
		err = errors.New("unexpected service type")
	}
	if err != nil {
		return fmt.Errorf("create describer for service type %s: %w", o.targetSvc.Type, err)
	}

	uri, err := svcDescriber.URI(o.targetEnvironment.Name)
	if err != nil {
		return fmt.Errorf("get uri for environment %s: %w", o.targetEnvironment.Name, err)
	}
	switch o.targetSvc.Type {
	case manifest.BackendServiceType:
		log.Successf("Deployed %s, its service discovery endpoint is %s.\n", color.HighlightUserInput(o.Name), color.HighlightResource(uri))
	default:
		log.Successf("Deployed %s, you can access it at %s.\n", color.HighlightUserInput(o.Name), color.HighlightResource(uri))
	}
	return nil
}

// BuildSvcDeployCmd builds the `svc deploy` subcommand.
func BuildSvcDeployCmd() *cobra.Command {
	vars := deploySvcVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a service to an environment.",
		Long:  `Deploys a service to an environment.`,
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot svc deploy --name frontend --env test
  Deploys a service with additional resource tags.
  /code $ copilot svc deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcDeployOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.ImageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.ResourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
