// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	awscloudformation "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/s3"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/tags"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	inputImageTagPrompt = "Input an image tag value:"
)

var (
	errNoLocalManifestsFound = errors.New("no manifest files found")
)

type appDeployVars struct {
	*GlobalOpts
	AppName      string
	EnvName      string
	ImageTag     string
	ResourceTags map[string]string
}

type appDeployOpts struct {
	appDeployVars

	projectService   projectService
	workspaceService wsAppReader
	ecrService       ecrService
	dockerService    dockerService
	s3Service        artifactUploader
	runner           runner
	addonsSvc        templater
	projectCFSvc     projectResourcesGetter
	appCFSvc         cloudformation.CloudFormation
	sessProvider     sessionProvider

	spinner progress

	// cached variables
	targetEnvironment *archer.Environment
	targetProject     *archer.Project
	targetApplication *archer.Application
}

func newAppDeployOpts(vars appDeployVars) (*appDeployOpts, error) {
	projectService, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("create project service: %w", err)
	}

	workspaceService, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("intialize workspace service: %w", err)
	}

	return &appDeployOpts{
		appDeployVars: vars,

		projectService:   projectService,
		workspaceService: workspaceService,
		spinner:          termprogress.NewSpinner(),
		dockerService:    docker.New(),
		runner:           command.New(),
		sessProvider:     session.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *appDeployOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if o.AppName != "" {
		if err := o.validateAppName(); err != nil {
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
func (o *appDeployOpts) Ask() error {
	if err := o.askAppName(); err != nil {
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

// Execute builds and pushes the container image for the application,
func (o *appDeployOpts) Execute() error {
	env, err := o.targetEnv()
	if err != nil {
		return err
	}
	o.targetEnvironment = env

	proj, err := o.projectService.GetApplication(o.ProjectName())
	if err != nil {
		return err
	}
	o.targetProject = proj

	app, err := o.projectService.GetService(o.ProjectName(), o.AppName)
	if err != nil {
		return fmt.Errorf("get application metadata: %w", err)
	}
	o.targetApplication = app

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

	if err := o.deployApp(addonsURL); err != nil {
		return err
	}

	return o.showAppURI()
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *appDeployOpts) RecommendedActions() []string {
	return nil
}

func (o *appDeployOpts) validateAppName() error {
	names, err := o.workspaceService.ServiceNames()
	if err != nil {
		return fmt.Errorf("list applications in the workspace: %w", err)
	}
	for _, name := range names {
		if o.AppName == name {
			return nil
		}
	}
	return fmt.Errorf("application %s not found in the workspace", color.HighlightUserInput(o.AppName))
}

func (o *appDeployOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *appDeployOpts) targetEnv() (*archer.Environment, error) {
	env, err := o.projectService.GetEnvironment(o.ProjectName(), o.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from metadata store: %w", o.EnvName, err)
	}
	return env, nil
}

func (o *appDeployOpts) askAppName() error {
	if o.AppName != "" {
		return nil
	}

	names, err := o.workspaceService.ServiceNames()
	if err != nil {
		return fmt.Errorf("list applications in workspace: %w", err)
	}
	if len(names) == 0 {
		return errors.New("no applications found in the workspace")
	}
	if len(names) == 1 {
		o.AppName = names[0]
		log.Infof("Only found one app, defaulting to: %s\n", color.HighlightUserInput(o.AppName))
		return nil
	}

	selectedAppName, err := o.prompt.SelectOne("Select an application", "", names)
	if err != nil {
		return fmt.Errorf("select app name: %w", err)
	}
	o.AppName = selectedAppName
	return nil
}

func (o *appDeployOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	envs, err := o.projectService.ListEnvironments(o.ProjectName())
	if err != nil {
		return fmt.Errorf("get environments for project %s from metadata store: %w", o.ProjectName(), err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with project %s, try initializing one: %s\n",
			color.HighlightUserInput(o.ProjectName()),
			color.HighlightCode("ecs-preview env init"))
		return fmt.Errorf("no environments found in project %s", o.ProjectName())
	}
	if len(envs) == 1 {
		o.EnvName = envs[0].Name
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(o.EnvName))
		return nil
	}

	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}

	selectedEnvName, err := o.prompt.SelectOne("Select an environment", "", names)
	if err != nil {
		return fmt.Errorf("select env name: %w", err)
	}
	o.EnvName = selectedEnvName
	return nil
}

func (o *appDeployOpts) askImageTag() error {
	if o.ImageTag != "" {
		return nil
	}

	tag, err := getVersionTag(o.runner)

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

func (o *appDeployOpts) configureClients() error {
	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", o.targetEnvironment.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(o.targetEnvironment.ManagerRoleARN, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("assuming environment manager role: %w", err)
	}

	// ECR client against tools account profile AND target environment region
	o.ecrService = ecr.New(defaultSessEnvRegion)

	o.s3Service = s3.New(defaultSessEnvRegion)

	// app deploy CF client against env account profile AND target environment region
	o.appCFSvc = cloudformation.New(envSession)

	addonsSvc, err := addons.New(o.AppName)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addonsSvc = addonsSvc

	// client to retrieve a project's resources created with CloudFormation
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	o.projectCFSvc = cloudformation.New(defaultSess)
	return nil
}

func (o *appDeployOpts) pushToECRRepo() error {
	repoName := fmt.Sprintf("%s/%s", o.projectName, o.AppName)

	uri, err := o.ecrService.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("get ECR repository URI: %w", err)
	}

	appDockerfilePath, err := o.getAppDockerfilePath()
	if err != nil {
		return err
	}

	if err := o.dockerService.Build(uri, o.ImageTag, appDockerfilePath); err != nil {
		return fmt.Errorf("build Dockerfile at %s with tag %s: %w", appDockerfilePath, o.ImageTag, err)
	}

	auth, err := o.ecrService.GetECRAuth()

	if err != nil {
		return fmt.Errorf("get ECR auth data: %w", err)
	}

	o.dockerService.Login(uri, auth.Username, auth.Password)

	return o.dockerService.Push(uri, o.ImageTag)
}

func (o *appDeployOpts) getAppDockerfilePath() (string, error) {
	type dfPath interface {
		DockerfilePath() string
	}

	manifestBytes, err := o.workspaceService.ReadServiceManifest(o.AppName)
	if err != nil {
		return "", fmt.Errorf("read manifest file %s: %w", o.AppName, err)
	}

	app, err := manifest.UnmarshalApp(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("unmarshal app manifest: %w", err)
	}

	mf, ok := app.(dfPath)
	if !ok {
		return "", fmt.Errorf("application %s does not have a dockerfile path", o.AppName)
	}
	return strings.TrimSuffix(mf.DockerfilePath(), "/Dockerfile"), nil
}

// pushAddonsTemplateToS3Bucket generates the addons template for the application and pushes it to S3.
// If the application doesn't have any addons, it returns the empty string and no errors.
// If the application has addons, it returns the URL of the S3 object storing the addons template.
func (o *appDeployOpts) pushAddonsTemplateToS3Bucket() (string, error) {
	template, err := o.addonsSvc.Template()
	if err != nil {
		var notExistErr *addons.ErrDirNotExist
		if errors.As(err, &notExistErr) {
			// addons doesn't exist for app, the url is empty.
			return "", nil
		}
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}
	resources, err := o.projectCFSvc.GetProjectResourcesByRegion(o.targetProject, o.targetEnvironment.Region)
	if err != nil {
		return "", fmt.Errorf("get project resources: %w", err)
	}

	reader := strings.NewReader(template)
	url, err := o.s3Service.PutArtifact(resources.S3Bucket, fmt.Sprintf(archer.AddonsCfnTemplateNameFormat, o.AppName), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", resources.S3Bucket, err)
	}
	return url, nil
}

func (o *appDeployOpts) manifest() (interface{}, error) {
	raw, err := o.workspaceService.ReadServiceManifest(o.AppName)
	if err != nil {
		return nil, fmt.Errorf("read app %s manifest from workspace: %w", o.AppName, err)
	}
	mft, err := manifest.UnmarshalApp(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal app %s manifest: %w", o.AppName, err)
	}
	return mft, nil
}

func (o *appDeployOpts) runtimeConfig(addonsURL string) (*stack.RuntimeConfig, error) {
	resources, err := o.projectCFSvc.GetProjectResourcesByRegion(o.targetProject, o.targetEnvironment.Region)
	if err != nil {
		return nil, fmt.Errorf("get project %s resources from region %s: %w", o.targetProject.Name, o.targetEnvironment.Region, err)
	}
	repoURL, ok := resources.RepositoryURLs[o.AppName]
	if !ok {
		return nil, &errRepoNotFound{
			appName:       o.AppName,
			envRegion:     o.targetEnvironment.Region,
			projAccountID: o.targetProject.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		ImageRepoURL:      repoURL,
		ImageTag:          o.ImageTag,
		AddonsTemplateURL: addonsURL,
		AdditionalTags:    tags.Merge(o.targetProject.Tags, o.ResourceTags),
	}, nil
}

func (o *appDeployOpts) stackConfiguration(addonsURL string) (cloudformation.StackConfiguration, error) {
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
	case *manifest.LoadBalancedWebApp:
		if o.targetProject.RequiresDNSDelegation() {
			conf, err = stack.NewHTTPSLoadBalancedWebApp(t, o.targetEnvironment.Name, o.targetEnvironment.Project, *rc)
		} else {
			conf, err = stack.NewLoadBalancedWebApp(t, o.targetEnvironment.Name, o.targetEnvironment.Project, *rc)
		}
	case *manifest.BackendApp:
		conf, err = stack.NewBackendApp(t, o.targetEnvironment.Name, o.targetEnvironment.Project, *rc)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

func (o *appDeployOpts) deployApp(addonsURL string) error {
	conf, err := o.stackConfiguration(addonsURL)
	if err != nil {
		return err
	}
	o.spinner.Start(
		fmt.Sprintf("Deploying %s to %s.",
			fmt.Sprintf("%s:%s", color.HighlightUserInput(o.AppName), color.HighlightUserInput(o.ImageTag)),
			color.HighlightUserInput(o.targetEnvironment.Name)))

	if err := o.appCFSvc.DeployApp(conf, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		o.spinner.Stop("Error!")
		return fmt.Errorf("deploy application: %w", err)
	}
	o.spinner.Stop("")
	return nil
}

func (o *appDeployOpts) showAppURI() error {
	type identifier interface {
		URI(string) (string, error)
	}

	var appDescriber identifier
	var err error
	switch o.targetApplication.Type {
	case manifest.LoadBalancedWebApplication:
		appDescriber, err = describe.NewWebAppDescriber(o.ProjectName(), o.AppName)
	case manifest.BackendApplication:
		appDescriber, err = describe.NewBackendAppDescriber(o.ProjectName(), o.AppName)
	default:
		err = errors.New("unexpected application type")
	}
	if err != nil {
		return fmt.Errorf("create describer for app type %s: %w", o.targetApplication.Type, err)
	}

	uri, err := appDescriber.URI(o.targetEnvironment.Name)
	if err != nil {
		return fmt.Errorf("get uri for environment %s: %w", o.targetEnvironment.Name, err)
	}
	switch o.targetApplication.Type {
	case manifest.BackendApplication:
		log.Successf("Deployed %s, its service discovery endpoint is %s.\n", color.HighlightUserInput(o.AppName), color.HighlightResource(uri))
	default:
		log.Successf("Deployed %s, you can access it at %s.\n", color.HighlightUserInput(o.AppName), color.HighlightResource(uri))
	}
	return nil
}

// BuildAppDeployCmd builds the `app deploy` subcommand.
func BuildAppDeployCmd() *cobra.Command {
	vars := appDeployVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an application to an environment.",
		Long:  `Deploys an application to an environment.`,
		Example: `
  Deploys an application named "frontend" to a "test" environment.
  /code $ ecs-preview app deploy --name frontend --env test
  Deploys an application with additional resource tags.
  /code $ ecs-preview app deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newAppDeployOpts(vars)
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
	cmd.Flags().StringVarP(&vars.AppName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.ImageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.ResourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
