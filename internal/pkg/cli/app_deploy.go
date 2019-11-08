// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

// BuildAppDeployCommand builds the `app deploy` subcommand.
func BuildAppDeployCommand() *cobra.Command {
	input := &appDeployOpts{
		GlobalOpts: NewGlobalOpts(),
		prompt:     prompt.New(),
		spinner:    termprogress.NewSpinner(),
	}

	cmd := &cobra.Command{
		Use:  "deploy",
		Long: `Deploy an application to an environment.`,
		Example: `
  Deploy an application named "frontend" to a "test" environment.
  /code $ archer app deploy --name frontend --env test`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := input.init(); err != nil {
				return err
			}

			if err := input.sourceInputs(); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := input.deployApp(); err != nil {
				return err
			}

			return nil
		},
		// PostRunE: func(cmd *cobra.Command, args []string) error {
		// TODO: recommended actions?
		// },
	}

	cmd.Flags().StringVarP(&input.app, "name", "n", "", "application name")
	cmd.Flags().StringVarP(&input.env, "env", "e", "", "environment name")
	cmd.Flags().StringVarP(&input.imageTag, "tag", "t", "", "image tag")

	return cmd
}

type appDeployOpts struct {
	*GlobalOpts
	app      string
	env      string
	imageTag string

	projectService   projectService
	ecrService       ecr.Service
	workspaceService *workspace.Workspace

	prompt  prompter
	spinner progress

	projectApplications []*archer.Application
	projectEnvironments []*archer.Environment
}

func (opts appDeployOpts) String() string {
	return fmt.Sprintf("project: %s, app: %s, env: %s, tag: %s", opts.ProjectName(), opts.app, opts.env, opts.imageTag)
}

type projectService interface {
	archer.ProjectStore
	archer.EnvironmentStore
	archer.ApplicationStore
}

func (opts *appDeployOpts) init() error {
	projectService, err := ssm.NewStore()
	if err != nil {
		return fmt.Errorf("create project service: %w", err)
	}
	opts.projectService = projectService

	// TODO: toolsAccountSession may need to be regionalized?
	toolsAccountSession, err := session.Default()
	if err != nil {
		return fmt.Errorf("initialize tools account session: %w", err)
	}
	opts.ecrService = ecr.New(toolsAccountSession)

	workspaceService, err := workspace.New()
	if err != nil {
		return fmt.Errorf("intialize workspace service: %w", err)
	}
	opts.workspaceService = workspaceService

	return nil
}

func (opts *appDeployOpts) sourceInputs() error {
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	if err := opts.sourceProjectData(); err != nil {
		return err
	}

	if err := opts.sourceAppName(); err != nil {
		return err
	}

	if err := opts.sourceEnvName(); err != nil {
		return err
	}

	if err := opts.sourceImageTag(); err != nil {
		return err
	}

	return nil
}

func (opts *appDeployOpts) sourceProjectData() error {
	if err := opts.sourceProjectApplications(); err != nil {
		return err
	}

	if err := opts.sourceProjectEnvironments(); err != nil {
		return err
	}

	return nil
}

func (opts *appDeployOpts) sourceProjectApplications() error {
	apps, err := opts.projectService.ListApplications(opts.ProjectName())

	if err != nil {
		return fmt.Errorf("get apps: %w", err)
	}

	if len(apps) == 0 {
		// TODO: recommend follow up command - app init?
		return errors.New("no applications found")
	}

	opts.projectApplications = apps

	return nil
}

func (opts *appDeployOpts) sourceProjectEnvironments() error {
	envs, err := opts.projectService.ListEnvironments(opts.ProjectName())

	if err != nil {
		return fmt.Errorf("get environments: %w", err)
	}

	if len(envs) == 0 {
		// TODO: recommend follow up command - env init?
		log.Infof("couldn't find any environments associated with project %s, try initializing one: %s\n",
			color.HighlightUserInput(opts.ProjectName()),
			color.HighlightCode("archer env init"))

		return errors.New("no environments found")
	}

	opts.projectEnvironments = envs

	return nil
}

func (opts *appDeployOpts) sourceAppName() error {
	appNames := []string{}

	// TODO: limit application options to only those in the local workspace
	for _, app := range opts.projectApplications {
		appNames = append(appNames, app.Name)
	}

	if opts.app == "" {
		if len(appNames) == 1 {
			opts.app = appNames[0]

			// NOTE: defaulting the app name, tell the user
			log.Infof("Only found one app, defaulting to: %s\n", color.HighlightUserInput(opts.app))

			return nil
		}

		selectedAppName, err := opts.prompt.SelectOne("Select an application", "", appNames)

		if err != nil {
			return fmt.Errorf("select app name: %w", err)
		}

		opts.app = selectedAppName
	}

	for _, appName := range appNames {
		if opts.app == appName {
			return nil
		}
	}

	return fmt.Errorf("invalid app name")
}

func (opts *appDeployOpts) sourceEnvName() error {
	envNames := []string{}

	for _, env := range opts.projectEnvironments {
		envNames = append(envNames, env.Name)
	}

	if opts.env == "" {
		if len(envNames) == 1 {
			opts.env = envNames[0]

			// NOTE: defaulting the env name, tell the user.
			log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(opts.env))

			return nil
		}

		selectedEnvName, err := opts.prompt.SelectOne("Select an environment", "", envNames)

		if err != nil {
			return fmt.Errorf("select env name: %w", err)
		}

		opts.env = selectedEnvName
	}

	for _, envName := range envNames {
		if opts.env == envName {
			return nil
		}
	}

	return fmt.Errorf("invalid env name")
}

func (opts *appDeployOpts) sourceImageTag() error {
	if opts.imageTag != "" {
		return nil
	}

	cmd := exec.Command("git", "describe", "--always")

	bytes, err := cmd.Output()

	if err != nil {
		return fmt.Errorf("defaulting tag: %w", err)
	}

	// NOTE: `git describe` output bytes includes a `\n` character, so we trim it out.
	opts.imageTag = strings.TrimSpace(string(bytes))

	return nil
}

func (opts appDeployOpts) deployApp() error {
	// TODO: remove ECR repository creation
	// Ideally this `getRepositoryURI` flow will not create an ECR repository, one will exist after the `app init` workflow.
	uri, err := getRepositoryURI(opts.ProjectName(), opts.app)
	if err != nil {
		return fmt.Errorf("get repository URI: %w", err)
	}

	dockerService := docker.New(uri)

	appDockerfilePath, err := opts.getAppDockerfilePath()
	if err != nil {
		return err
	}

	if err := dockerService.Build(opts.imageTag, appDockerfilePath); err != nil {
		return fmt.Errorf("build Dockerfile at %s with tag %s, %w", appDockerfilePath, opts.imageTag, err)
	}

	auth, err := opts.ecrService.GetECRAuth()

	if err != nil {
		return fmt.Errorf("get ECR auth data: %w", err)
	}

	dockerService.Login(auth)

	if err != nil {
		return err
	}

	if err = dockerService.Push(opts.imageTag); err != nil {
		return err
	}

	template, err := opts.getAppDeployTemplate()

	stackName := fmt.Sprintf("%s-%s", opts.app, opts.env)
	changeSetName := fmt.Sprintf("%s-%s", stackName, opts.imageTag)

	opts.spinner.Start(fmt.Sprintf("Deploying %s to %s.",
		fmt.Sprintf("%s:%s", color.HighlightUserInput(opts.app),
			color.HighlightUserInput(opts.imageTag)),
		color.HighlightUserInput(opts.env)))

	if err := applyAppDeployTemplate(template, stackName, changeSetName); err != nil {
		opts.spinner.Stop("Error!")

		return err
	}

	opts.spinner.Stop("Done!")

	log.Successf("Deployed %s to %s.\n", fmt.Sprintf("%s:%s", color.HighlightUserInput(opts.app),
		color.HighlightUserInput(opts.imageTag)),
		color.HighlightUserInput(opts.env))

	return nil
}

func (opts appDeployOpts) getAppDeployTemplate() (string, error) {
	buffer := &bytes.Buffer{}

	appPackage := PackageAppOpts{
		AppName:      opts.app,
		EnvName:      opts.env,
		Tag:          opts.imageTag,
		stackWriter:  buffer,
		paramsWriter: ioutil.Discard,
		envStore:     opts.projectService,
		ws:           opts.workspaceService,
		GlobalOpts:   opts.GlobalOpts,
	}

	if err := appPackage.Execute(); err != nil {
		return "", fmt.Errorf("package application: %w", err)
	}

	return buffer.String(), nil
}

func applyAppDeployTemplate(template, stackName, changeSetName string) error {
	// TODO: create a session from the environment profile to support cross-account?
	session, err := session.Default()
	if err != nil {
		// TODO: handle err
		return err
	}

	cfClient := cloudformation.New(session)

	if err := cfClient.DeployApp(template, stackName, changeSetName); err != nil {
		return fmt.Errorf("deploy application: %w", err)
	}

	return nil
}

func getRepositoryURI(projectName, appName string) (string, error) {
	sess, err := session.Default()

	if err != nil {
		return "", err
	}

	ecrService := ecr.New(sess)

	// assume the ECR repository name is the projectName/appName
	repoName := fmt.Sprintf("%s/%s", projectName, appName)

	// try to describe the repository to see if it exists
	// NOTE: this should go away once ECR repositories are managed elsewhere
	uri, err := ecrService.GetRepository(repoName)
	// if there was an error assume the repo doesn't exist and try to create it
	if err == nil {
		return uri, nil
	}

	uri, err = ecrService.CreateRepository(repoName)

	if err != nil {
		return "", err
	}

	return uri, nil
}

func (opts appDeployOpts) getAppDockerfilePath() (string, error) {
	manifestFileNames, err := opts.workspaceService.ListManifestFiles()
	if err != nil {
		return "", err
	}
	if len(manifestFileNames) == 0 {
		return "", errors.New("no manifest files found")
	}

	var targetManifestFile string
	for _, f := range manifestFileNames {
		if strings.Contains(f, opts.app) {
			targetManifestFile = f
			break
		}
	}
	if targetManifestFile == "" {
		return "", errors.New("couldn't match manifest file name")
	}

	manifestBytes, err := opts.workspaceService.ReadManifestFile(targetManifestFile)
	if err != nil {
		return "", err
	}

	mf, err := manifest.UnmarshalApp(manifestBytes)
	if err != nil {
		return "", err
	}

	return mf.DockerfilePath(), nil
}
