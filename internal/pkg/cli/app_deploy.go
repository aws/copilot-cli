// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

type appDeployOpts struct {
	*GlobalOpts
	AppName  string
	EnvName  string
	ImageTag string

	projectService     projectService
	workspaceService   archer.Workspace
	ecrService         ecrService
	dockerService      dockerService
	runner             runner
	appPackageCfClient projectResourcesGetter
	appDeployCfClient  cloudformation.CloudFormation
	sessProvider       sessionProvider

	spinner progress

	targetEnvironment *archer.Environment
}

func (opts *appDeployOpts) String() string {
	return fmt.Sprintf("project: %s, app: %s, env: %s, tag: %s", opts.ProjectName(), opts.AppName, opts.EnvName, opts.ImageTag)
}

func (opts *appDeployOpts) init() error {
	projectService, err := store.New()
	if err != nil {
		return fmt.Errorf("create project service: %w", err)
	}
	opts.projectService = projectService

	workspaceService, err := workspace.New()
	if err != nil {
		return fmt.Errorf("intialize workspace service: %w", err)
	}
	opts.workspaceService = workspaceService
	return nil
}

// Validate returns an error if the user inputs are invalid.
func (opts *appDeployOpts) Validate() error {
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if opts.AppName != "" {
		if err := opts.validateAppName(); err != nil {
			return err
		}
	}
	if opts.EnvName != "" {
		if err := opts.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (opts *appDeployOpts) Ask() error {
	if err := opts.askAppName(); err != nil {
		return err
	}
	if err := opts.askEnvName(); err != nil {
		return err
	}
	if err := opts.askImageTag(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the application,
func (opts *appDeployOpts) Execute() error {
	env, err := opts.targetEnv()
	if err != nil {
		return err
	}
	opts.targetEnvironment = env

	if err := opts.configureClients(); err != nil {
		return err
	}

	repoName := fmt.Sprintf("%s/%s", opts.projectName, opts.AppName)

	uri, err := opts.ecrService.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("get ECR repository URI: %w", err)
	}

	appDockerfilePath, err := opts.getAppDockerfilePath()
	if err != nil {
		return err
	}

	if err := opts.dockerService.Build(uri, opts.ImageTag, appDockerfilePath); err != nil {
		return fmt.Errorf("build Dockerfile at %s with tag %s: %w", appDockerfilePath, opts.ImageTag, err)
	}

	auth, err := opts.ecrService.GetECRAuth()

	if err != nil {
		return fmt.Errorf("get ECR auth data: %w", err)
	}

	opts.dockerService.Login(uri, auth.Username, auth.Password)

	if err != nil {
		return err
	}

	if err = opts.dockerService.Push(uri, opts.ImageTag); err != nil {
		return err
	}

	template, err := opts.getAppDeployTemplate()

	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate random id for changeSet: %w", err)
	}
	stackName := stack.NameForApp(opts.ProjectName(), opts.targetEnvironment.Name, opts.AppName)
	changeSetName := fmt.Sprintf("%s-%s", stackName, id)

	opts.spinner.Start(
		fmt.Sprintf("Deploying %s to %s.",
			fmt.Sprintf("%s:%s", color.HighlightUserInput(opts.AppName), color.HighlightUserInput(opts.ImageTag)),
			color.HighlightUserInput(opts.targetEnvironment.Name)))

	// TODO Use the Tags() method defined in deploy/cloudformation/stack/lb_fargate_app.go
	tags := map[string]string{
		stack.ProjectTagKey: opts.ProjectName(),
		stack.EnvTagKey:     opts.targetEnvironment.Name,
		stack.AppTagKey:     opts.AppName,
	}
	if err := opts.applyAppDeployTemplate(template, stackName, changeSetName, opts.targetEnvironment.ExecutionRoleARN, tags); err != nil {
		opts.spinner.Stop("Error!")
		return err
	}
	opts.spinner.Stop("")

	identifier, err := describe.NewWebAppDescriber(opts.ProjectName(), opts.AppName)
	if err != nil {
		return fmt.Errorf("create identifier for application %s in project %s: %w", opts.AppName, opts.ProjectName(), err)
	}
	loadBalancerURI, err := identifier.URI(opts.targetEnvironment.Name)
	if err != nil {
		return fmt.Errorf("cannot retrieve the URI from environment %s: %w", opts.EnvName, err)
	}
	log.Successf("Deployed %s, you can access it at %s\n", color.HighlightUserInput(opts.AppName), loadBalancerURI.String())
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (opts *appDeployOpts) RecommendedActions() []string {
	return nil
}

func (opts *appDeployOpts) validateAppName() error {
	names, err := opts.workspaceAppNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if opts.AppName == name {
			return nil
		}
	}
	return fmt.Errorf("application %s not found in the workspace", color.HighlightUserInput(opts.AppName))
}

func (opts *appDeployOpts) validateEnvName() error {
	if _, err := opts.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (opts *appDeployOpts) targetEnv() (*archer.Environment, error) {
	env, err := opts.projectService.GetEnvironment(opts.ProjectName(), opts.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from metadata store: %w", opts.EnvName, err)
	}
	return env, nil
}

func (opts *appDeployOpts) workspaceAppNames() ([]string, error) {
	apps, err := opts.workspaceService.Apps()
	if err != nil {
		return nil, fmt.Errorf("get applications in the workspace: %w", err)
	}
	var names []string
	for _, app := range apps {
		names = append(names, app.AppName())
	}
	return names, nil
}

func (opts *appDeployOpts) askAppName() error {
	if opts.AppName != "" {
		return nil
	}

	names, err := opts.workspaceAppNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("no applications found in the workspace")
	}
	if len(names) == 1 {
		opts.AppName = names[0]
		log.Infof("Only found one app, defaulting to: %s\n", color.HighlightUserInput(opts.AppName))
		return nil
	}

	selectedAppName, err := opts.prompt.SelectOne("Select an application", "", names)
	if err != nil {
		return fmt.Errorf("select app name: %w", err)
	}
	opts.AppName = selectedAppName
	return nil
}

func (opts *appDeployOpts) askEnvName() error {
	if opts.EnvName != "" {
		return nil
	}

	envs, err := opts.projectService.ListEnvironments(opts.ProjectName())
	if err != nil {
		return fmt.Errorf("get environments for project %s from metadata store: %w", opts.ProjectName(), err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with project %s, try initializing one: %s\n",
			color.HighlightUserInput(opts.ProjectName()),
			color.HighlightCode("ecs-preview env init"))
		return fmt.Errorf("no environments found in project %s", opts.ProjectName())
	}
	if len(envs) == 1 {
		opts.EnvName = envs[0].Name
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(opts.EnvName))
		return nil
	}

	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}

	selectedEnvName, err := opts.prompt.SelectOne("Select an environment", "", names)
	if err != nil {
		return fmt.Errorf("select env name: %w", err)
	}
	opts.EnvName = selectedEnvName
	return nil
}

func (opts *appDeployOpts) askImageTag() error {
	if opts.ImageTag != "" {
		return nil
	}

	tag, err := getVersionTag(opts.runner)

	if err == nil {
		opts.ImageTag = tag

		return nil
	}

	log.Warningln("Failed to default tag, are you in a git repository?")

	userInputTag, err := opts.prompt.Get(inputImageTagPrompt, "", nil /*no validation*/)
	if err != nil {
		return fmt.Errorf("prompt for image tag: %w", err)
	}

	opts.ImageTag = userInputTag

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

func (opts *appDeployOpts) getAppDeployTemplate() (string, error) {
	buffer := &bytes.Buffer{}

	appPackage := packageAppOpts{
		AppName:      opts.AppName,
		EnvName:      opts.targetEnvironment.Name,
		Tag:          opts.ImageTag,
		stackWriter:  buffer,
		paramsWriter: ioutil.Discard,
		store:        opts.projectService,
		describer:    opts.appPackageCfClient,
		ws:           opts.workspaceService,
		GlobalOpts:   opts.GlobalOpts,
	}

	if err := appPackage.Execute(); err != nil {
		return "", fmt.Errorf("package application: %w", err)
	}
	return buffer.String(), nil
}

func (opts *appDeployOpts) applyAppDeployTemplate(template, stackName, changeSetName, cfExecutionRole string, tags map[string]string) error {
	if err := opts.appDeployCfClient.DeployApp(template, stackName, changeSetName, cfExecutionRole, tags); err != nil {
		return fmt.Errorf("deploy application: %w", err)
	}
	return nil
}

func (opts *appDeployOpts) getAppDockerfilePath() (string, error) {
	manifestFileNames, err := opts.workspaceService.ListManifestFiles()
	if err != nil {
		return "", fmt.Errorf("list local manifest files: %w", err)
	}
	if len(manifestFileNames) == 0 {
		return "", errNoLocalManifestsFound
	}

	var targetManifestFile string
	for _, f := range manifestFileNames {
		if strings.Contains(f, opts.AppName) {
			targetManifestFile = f
			break
		}
	}
	if targetManifestFile == "" {
		return "", fmt.Errorf("couldn't find local manifest %s", opts.AppName)
	}

	manifestBytes, err := opts.workspaceService.ReadFile(targetManifestFile)
	if err != nil {
		return "", fmt.Errorf("read manifest file %s: %w", targetManifestFile, err)
	}

	mf, err := manifest.UnmarshalApp(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("unmarshal app manifest: %w", err)
	}

	return strings.TrimSuffix(mf.DockerfilePath(), "/Dockerfile"), nil
}

// BuildAppDeployCmd builds the `app deploy` subcommand.
func BuildAppDeployCmd() *cobra.Command {
	opts := &appDeployOpts{
		GlobalOpts:    NewGlobalOpts(),
		spinner:       termprogress.NewSpinner(),
		dockerService: docker.New(),
		runner:        command.New(),
		sessProvider:  session.NewProvider(),
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an application to an environment.",
		Long:  `Deploys an application to an environment.`,
		Example: `
  Deploys an application named "frontend" to a "test" environment.
  /code $ ecs-preview app deploy --name frontend --env test`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.init(); err != nil {
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
	cmd.Flags().StringVarP(&opts.AppName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&opts.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&opts.ImageTag, imageTagFlag, "", imageTagFlagDescription)

	return cmd
}
