// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployJobVars struct {
	appName      string
	name         string
	envName      string
	imageTag     string
	resourceTags map[string]string
}

type deployJobOpts struct {
	deployJobVars

	store              store
	ws                 wsJobDirReader
	unmarshal          func(in []byte) (interface{}, error)
	cmd                runner
	addons             templater
	appCFN             appResourcesGetter
	jobCFN             cloudformation.CloudFormation
	imageBuilderPusher imageBuilderPusher
	sessProvider       sessionProvider
	s3                 artifactUploader

	spinner progress
	sel     wsSelector
	prompt  prompter

	targetApp         *config.Application
	targetEnvironment *config.Environment
	targetJob         *config.Workload
	buildRequired     bool
}

func newJobDeployOpts(vars deployJobVars) (*deployJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	return &deployJobOpts{
		deployJobVars: vars,

		store:        store,
		ws:           ws,
		unmarshal:    manifest.UnmarshalWorkload,
		spinner:      termprogress.NewSpinner(),
		sel:          selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:       prompter,
		cmd:          command.New(),
		sessProvider: sessions.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deployJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		if err := o.validateJobName(); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deployJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	tag, err := askImageTag(o.imageTag, o.prompt, o.cmd)
	if err != nil {
		return err
	}
	o.imageTag = tag
	return nil
}

// Execute builds and pushes the container image for the job.
func (o *deployJobOpts) Execute() error {
	env, err := targetEnv(o.store, o.appName, o.envName)
	if err != nil {
		return err
	}
	o.targetEnvironment = env

	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}
	o.targetApp = app

	job, err := o.store.GetJob(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get job configuration: %w", err)
	}
	o.targetJob = job

	if err := o.configureClients(); err != nil {
		return err
	}

	if err := o.configureContainerImage(); err != nil {
		return err
	}

	addonsURL, err := o.pushAddonsTemplateToS3Bucket()
	if err != nil {
		return err
	}

	return o.deployJob(addonsURL)
}

// pushAddonsTemplateToS3Bucket generates the addons template for the job and pushes it to S3.
// If the job doesn't have any addons, it returns the empty string and no errors.
// If the job has addons, it returns the URL of the S3 object storing the addons template.
func (o *deployJobOpts) pushAddonsTemplateToS3Bucket() (string, error) {
	template, err := o.addons.Template()
	if err != nil {
		var notExistErr *addon.ErrDirNotExist
		if errors.As(err, &notExistErr) {
			// addons doesn't exist for job, the url is empty.
			return "", nil
		}
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return "", fmt.Errorf("get app resources: %w", err)
	}

	reader := strings.NewReader(template)
	url, err := o.s3.PutArtifact(resources.S3Bucket, fmt.Sprintf(config.AddonsCfnTemplateNameFormat, o.name), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", resources.S3Bucket, err)
	}
	return url, nil
}

func (o *deployJobOpts) configureClients() error {
	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", o.targetEnvironment.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(o.targetEnvironment.ManagerRoleARN, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("assuming environment manager role: %w", err)
	}

	// ECR client against tools account profile AND target environment region
	repoName := fmt.Sprintf("%s/%s", o.appName, o.name)
	registry := ecr.New(defaultSessEnvRegion)
	o.imageBuilderPusher, err = repository.New(repoName, registry)
	if err != nil {
		return fmt.Errorf("initiate image builder pusher: %w", err)
	}

	o.s3 = s3.New(defaultSessEnvRegion)

	// CF client against env account profile AND target environment region
	o.jobCFN = cloudformation.New(envSession)

	addonsSvc, err := addon.New(o.name)
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

func (o *deployJobOpts) configureContainerImage() error {
	job, err := o.manifest()
	if err != nil {
		return err
	}
	required, err := manifest.JobDockerfileBuildRequired(job)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	// If it is built from local Dockerfile, build and push to the ECR repo.
	buildArg, err := o.dfBuildArgs(job)
	if err != nil {
		return err
	}
	if err := o.imageBuilderPusher.BuildAndPush(docker.New(), buildArg); err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	o.buildRequired = true
	return nil
}

func (o *deployJobOpts) dfBuildArgs(job interface{}) (*docker.BuildArguments, error) {
	copilotDir, err := o.ws.CopilotDirPath()
	if err != nil {
		return nil, fmt.Errorf("get copilot directory: %w", err)
	}
	return buildArgs(o.name, o.imageTag, copilotDir, job)
}

func (o *deployJobOpts) deployJob(addonsURL string) error {
	conf, err := o.stackConfiguration(addonsURL)
	if err != nil {
		return err
	}
	o.spinner.Start(
		fmt.Sprintf("Deploying %s to %s",
			fmt.Sprintf("%s:%s", color.HighlightUserInput(o.name), color.HighlightUserInput(o.imageTag)),
			color.HighlightUserInput(o.targetEnvironment.Name),
		),
	)
	if err := o.jobCFN.DeployService(conf, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		o.spinner.Stop(log.Serrorf("Failed to deploy job.\n"))
		return fmt.Errorf("deploy job: %w", err)
	}
	o.spinner.Stop("\n")
	return nil
}

func (o *deployJobOpts) stackConfiguration(addonsURL string) (cloudformation.StackConfiguration, error) {
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
	case *manifest.ScheduledJob:
		conf, err = stack.NewScheduledJob(t, o.targetEnvironment.Name, o.targetEnvironment.App, *rc)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

func (o *deployJobOpts) runtimeConfig(addonsURL string) (*stack.RuntimeConfig, error) {
	if !o.buildRequired {
		return &stack.RuntimeConfig{
			AddonsTemplateURL: addonsURL,
			AdditionalTags:    tags.Merge(o.targetApp.Tags, o.resourceTags),
		}, nil
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return nil, fmt.Errorf("get application %s resources from region %s: %w", o.targetApp.Name, o.targetEnvironment.Region, err)
	}
	repoURL, ok := resources.RepositoryURLs[o.name]
	if !ok {
		return nil, &errRepoNotFound{
			svcName:      o.name,
			envRegion:    o.targetEnvironment.Region,
			appAccountID: o.targetApp.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		Image: &stack.ECRImage{
			RepoURL:  repoURL,
			ImageTag: o.imageTag,
		},
		AddonsTemplateURL: addonsURL,
		AdditionalTags:    tags.Merge(o.targetApp.Tags, o.resourceTags),
	}, nil
}

func (o *deployJobOpts) manifest() (interface{}, error) {
	raw, err := o.ws.ReadJobManifest(o.name)
	if err != nil {
		return nil, fmt.Errorf("read job %s manifest: %w", o.name, err)
	}
	mft, err := o.unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal job %s manifest: %w", o.name, err)
	}
	return mft, nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deployJobOpts) RecommendedActions() []string {
	return nil
}

func (o *deployJobOpts) validateJobName() error {
	names, err := o.ws.JobNames()
	if err != nil {
		return fmt.Errorf("list jobs in the workspace: %w", err)
	}
	for _, name := range names {
		if o.name == name {
			return nil
		}
	}
	return fmt.Errorf("job %s not found in the workspace", color.HighlightUserInput(o.name))
}

func (o *deployJobOpts) validateEnvName() error {
	if _, err := targetEnv(o.store, o.appName, o.envName); err != nil {
		return err
	}
	return nil
}

func (o *deployJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job("Select a job from your workspace", "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *deployJobOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deployJobOpts) askImageTag() error {
	if o.imageTag != "" {
		return nil
	}

	tag, err := getVersionTag(o.cmd)

	if err == nil {
		o.imageTag = tag

		return nil
	}

	log.Warningln("Failed to default tag, are you in a git repository?")

	userInputTag, err := o.prompt.Get(inputImageTagPrompt, "", prompt.RequireNonEmpty)
	if err != nil {
		return fmt.Errorf("prompt for image tag: %w", err)
	}
	o.imageTag = userInputTag
	return nil
}

// buildJobDeployCmd builds the `job deploy` subcommand.
func buildJobDeployCmd() *cobra.Command {
	vars := deployJobVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a job to an environment.",
		Long:  `Deploys a job to an environment.`,
		Example: `
  Deploys a job named "report-gen" to a "test" environment.
  /code $ copilot job deploy --name report-gen --env test
  Deploys a job with additional resource tags.
  /code $ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newJobDeployOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
