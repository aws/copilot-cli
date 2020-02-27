// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/s3"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/google/uuid"
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
	AppName  string
	EnvName  string
	ImageTag string

	// Feature flags -- these flags are not enabled while building the binary.
	// However, it allows us to test our codebase until the feature is ready.
	enableAddons bool
}

type appDeployOpts struct {
	appDeployVars

	projectService     projectService
	workspaceService   wsAppReader
	ecrService         ecrService
	dockerService      dockerService
	s3Service          artifactPutter
	runner             runner
	appPackageCfClient projectResourcesGetter
	appDeployCfClient  cloudformation.CloudFormation
	sessProvider       sessionProvider

	spinner progress

	// cached variables
	targetEnvironment *archer.Environment
}

type cfnTemplates struct {
	app    *bytes.Buffer
	addons *bytes.Buffer
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

	if err := o.configureClients(); err != nil {
		return err
	}

	if err := o.pushToECRRepo(); err != nil {
		return err
	}

	template, err := o.retrieveTemplate()
	if err != nil {
		return err
	}

	// TODO: delete addons template from S3 bucket when deleting the environment.
	addonsURL, err := o.pushAddonsTemplateToS3Bucket(template.addons)
	if err != nil {
		return err
	}

	if err := o.deployAppStack(template.app, addonsURL); err != nil {
		return err
	}

	return o.showAppURI()
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *appDeployOpts) RecommendedActions() []string {
	return nil
}

func (o *appDeployOpts) validateAppName() error {
	names, err := o.workspaceService.AppNames()
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

	names, err := o.workspaceService.AppNames()
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
	o.appDeployCfClient = cloudformation.New(envSession)

	// app package CF client against tools account
	appPackageCfSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create app package CF session: %w", err)
	}
	o.appPackageCfClient = cloudformation.New(appPackageCfSess)
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
	manifestBytes, err := o.workspaceService.ReadAppManifest(o.AppName)
	if err != nil {
		return "", fmt.Errorf("read manifest file %s: %w", o.AppName, err)
	}

	mf, err := manifest.UnmarshalApp(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("unmarshal app manifest: %w", err)
	}

	return strings.TrimSuffix(mf.DockerfilePath(), "/Dockerfile"), nil
}

func (o *appDeployOpts) retrieveTemplate() (*cfnTemplates, error) {
	appBuffer := &bytes.Buffer{}
	addonsBuffer := &bytes.Buffer{}

	appPackage := packageAppOpts{
		packageAppVars: packageAppVars{
			AppName:    o.AppName,
			EnvName:    o.targetEnvironment.Name,
			Tag:        o.ImageTag,
			GlobalOpts: o.GlobalOpts,
		},

		stackWriter:   appBuffer,
		paramsWriter:  ioutil.Discard,
		addonsWriter:  addonsBuffer,
		initAddonsSvc: initPackageAddonsSvc,
		store:         o.projectService,
		describer:     o.appPackageCfClient,
		ws:            o.workspaceService,
	}

	if err := appPackage.Execute(); err != nil {
		return nil, fmt.Errorf("retrieve template: %w", err)
	}
	return &cfnTemplates{
		addons: addonsBuffer,
		app:    appBuffer,
	}, nil
}

func (o *appDeployOpts) pushAddonsTemplateToS3Bucket(addonsTemplate *bytes.Buffer) (string, error) {
	if !o.enableAddons || addonsTemplate.String() == "" {
		return "", nil
	}
	proj, err := o.projectService.GetProject(o.ProjectName())
	if err != nil {
		return "", fmt.Errorf("get project: %w", err)
	}
	resources, err := o.appPackageCfClient.GetProjectResourcesByRegion(proj, o.targetEnvironment.Region)
	if err != nil {
		return "", fmt.Errorf("get project resources: %w", err)
	}

	url, err := o.s3Service.PutArtifact(resources.S3Bucket, fmt.Sprintf(archer.AddonsCfnTemplateNameFormat, o.AppName), addonsTemplate)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", resources.S3Bucket, err)
	}
	return url, nil
}

func (o *appDeployOpts) deployAppStack(appTemplate *bytes.Buffer, addonsURL string) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate random id for changeSet: %w", err)
	}
	stackName := stack.NameForApp(o.ProjectName(), o.targetEnvironment.Name, o.AppName)
	changeSetName := fmt.Sprintf("%s-%s", stackName, id)

	o.spinner.Start(
		fmt.Sprintf("Deploying %s to %s.",
			fmt.Sprintf("%s:%s", color.HighlightUserInput(o.AppName), color.HighlightUserInput(o.ImageTag)),
			color.HighlightUserInput(o.targetEnvironment.Name)))

	// TODO Use the Tags() method defined in deploy/cloudformation/stack/lb_fargate_app.go
	tags := map[string]string{
		stack.ProjectTagKey: o.ProjectName(),
		stack.EnvTagKey:     o.targetEnvironment.Name,
		stack.AppTagKey:     o.AppName,
	}
	params := make(map[string]string)
	params["AddonsTemplateURL"] = addonsURL
	if err := o.appDeployCfClient.DeployApp(appTemplate.String(), stackName, changeSetName, o.targetEnvironment.ExecutionRoleARN, tags, params); err != nil {
		o.spinner.Stop("Error!")
		return fmt.Errorf("deploy application: %w", err)
	}
	o.spinner.Stop("")

	return nil
}

func (o *appDeployOpts) showAppURI() error {
	identifier, err := describe.NewWebAppDescriber(o.ProjectName(), o.AppName)
	if err != nil {
		return fmt.Errorf("create identifier for application %s in project %s: %w", o.AppName, o.ProjectName(), err)
	}
	loadBalancerURI, err := identifier.URI(o.targetEnvironment.Name)
	if err != nil {
		return fmt.Errorf("cannot retrieve the URI from environment %s: %w", o.EnvName, err)
	}
	log.Successf("Deployed %s, you can access it at %s\n", color.HighlightUserInput(o.AppName), loadBalancerURI.String())

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
  /code $ ecs-preview app deploy --name frontend --env test`,
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

	return cmd
}
