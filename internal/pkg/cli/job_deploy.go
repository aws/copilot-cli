// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/exec"
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
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployJobOpts struct {
	deployWkldVars

	store              store
	ws                 wsJobDirReader
	unmarshal          func(in []byte) (manifest.WorkloadManifest, error)
	newInterpolator    func(app, env string) interpolator
	cmd                runner
	addons             templater
	appCFN             appResourcesGetter
	jobCFN             cloudformation.CloudFormation
	imageBuilderPusher imageBuilderPusher
	sessProvider       sessionProvider
	s3                 artifactUploader
	envUpgradeCmd      actionCommand
	endpointGetter     endpointGetter

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	targetApp         *config.Application
	targetEnvironment *config.Environment
	targetJob         *config.Workload
	appEnvResources   *stack.AppRegionalResources
	appliedManifest   interface{}
	addonsURL         string
	envFileARN        string
	imageDigest       string
	buildRequired     bool
}

func newJobDeployOpts(vars deployWkldVars) (*deployJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	if err != nil {
		return nil, err
	}
	return &deployJobOpts{
		deployWkldVars: vars,

		store:           store,
		ws:              ws,
		unmarshal:       manifest.UnmarshalWorkload,
		spinner:         termprogress.NewSpinner(log.DiagnosticWriter),
		sel:             selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:          prompter,
		cmd:             exec.NewCmd(),
		sessProvider:    sessions.NewProvider(),
		newInterpolator: newManifestInterpolator,
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
	return nil
}

// Execute builds and pushes the container image for the job.
func (o *deployJobOpts) Execute() error {
	o.imageTag = imageTagFromGit(o.cmd, o.imageTag) // Best effort assign git tag.
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
	if err := o.envUpgradeCmd.Execute(); err != nil {
		return fmt.Errorf(`execute "env upgrade --app %s --name %s": %v`, o.appName, o.targetEnvironment.Name, err)
	}
	if err := o.configureContainerImage(); err != nil {
		return err
	}
	if err := o.pushArtifactsToS3(); err != nil {
		return err
	}
	return o.deployJob()
}

func (o *deployJobOpts) pushArtifactsToS3() error {
	mft, err := o.manifest()
	if err != nil {
		return err
	}
	if err := o.pushEnvFilesToS3Bucket(envFile(o.name, mft)); err != nil {
		return err
	}
	return o.pushAddonsTemplateToS3Bucket()
}

func (o *deployJobOpts) pushEnvFilesToS3Bucket(fileName string) error {
	if fileName == "" {
		return nil
	}
	content, err := o.ws.ReadSvcFile(o.name, fileName)
	if err != nil {
		return fmt.Errorf("read env file %s: %w", fileName, err)
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return err
	}
	reader := bytes.NewReader(content)
	url, err := o.s3.PutArtifact(o.appEnvResources.S3Bucket, fileName, reader)
	if err != nil {
		return fmt.Errorf("put env file %s artifact to bucket %s: %w", fileName, o.appEnvResources.S3Bucket, err)
	}
	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse s3 url: %w", err)
	}
	// The app and environment are always within the same partition.
	region := o.targetEnvironment.Region
	partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	if !ok {
		return fmt.Errorf("find the partition for region %s", region)
	}
	o.envFileARN = s3.FormatARN(partition.ID(), fmt.Sprintf("%s/%s", bucket, key))
	return nil
}

func (o *deployJobOpts) pushAddonsTemplateToS3Bucket() error {
	template, err := o.addons.Template()
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if errors.As(err, &notFoundErr) {
			// addons doesn't exist for job, the url is empty.
			return nil
		}
		return fmt.Errorf("retrieve addons template: %w", err)
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return err
	}
	reader := strings.NewReader(template)
	url, err := o.s3.PutArtifact(o.appEnvResources.S3Bucket, fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, o.name), reader)
	if err != nil {
		return fmt.Errorf("put addons artifact to bucket %s: %w", o.appEnvResources.S3Bucket, err)
	}
	o.addonsURL = url
	return nil
}

func (o *deployJobOpts) retrieveAppResourcesForEnvRegion() error {
	if o.appEnvResources != nil {
		return nil
	}
	resources, err := o.appCFN.GetAppResourcesByRegion(o.targetApp, o.targetEnvironment.Region)
	if err != nil {
		return fmt.Errorf("get application %s resources from region %s: %w", o.targetApp.Name, o.targetEnvironment.Region, err)
	}
	o.appEnvResources = resources
	return nil
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
	o.endpointGetter, err = describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return fmt.Errorf("initiate environment describer: %w", err)
	}

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

	cmd, err := newEnvUpgradeOpts(envUpgradeVars{
		appName: o.appName,
		name:    o.targetEnvironment.Name,
	})
	if err != nil {
		return fmt.Errorf("new env upgrade command: %v", err)
	}
	o.envUpgradeCmd = cmd
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
	digest, err := o.imageBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArg)
	if err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	o.imageDigest = digest
	o.buildRequired = true
	return nil
}

func (o *deployJobOpts) dfBuildArgs(job interface{}) (*dockerengine.BuildArguments, error) {
	copilotDir, err := o.ws.CopilotDirPath()
	if err != nil {
		return nil, fmt.Errorf("get copilot directory: %w", err)
	}
	return buildArgs(o.name, o.imageTag, copilotDir, job)
}

func (o *deployJobOpts) deployJob() error {
	conf, err := o.stackConfiguration()
	if err != nil {
		return err
	}
	if err := o.jobCFN.DeployService(os.Stderr, conf, awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)); err != nil {
		return fmt.Errorf("deploy job: %w", err)
	}
	log.Successf("Deployed %s.\n", color.HighlightUserInput(o.name))
	return nil
}

func (o *deployJobOpts) stackConfiguration() (cloudformation.StackConfiguration, error) {
	mft, err := o.manifest()
	if err != nil {
		return nil, err
	}
	rc, err := o.runtimeConfig()
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

func (o *deployJobOpts) runtimeConfig() (*stack.RuntimeConfig, error) {
	endpoint, err := o.endpointGetter.ServiceDiscoveryEndpoint()
	if err != nil {
		return nil, err
	}
	if !o.buildRequired {
		return &stack.RuntimeConfig{
			AddonsTemplateURL:        o.addonsURL,
			AdditionalTags:           tags.Merge(o.targetApp.Tags, o.resourceTags),
			ServiceDiscoveryEndpoint: endpoint,
			AccountID:                o.targetEnvironment.AccountID,
			Region:                   o.targetEnvironment.Region,
		}, nil
	}
	if err := o.retrieveAppResourcesForEnvRegion(); err != nil {
		return nil, err
	}
	repoURL, ok := o.appEnvResources.RepositoryURLs[o.name]
	if !ok {
		return nil, &errRepoNotFound{
			wlName:       o.name,
			envRegion:    o.targetEnvironment.Region,
			appAccountID: o.targetApp.AccountID,
		}
	}
	return &stack.RuntimeConfig{
		Image: &stack.ECRImage{
			RepoURL:  repoURL,
			ImageTag: o.imageTag,
			Digest:   o.imageDigest,
		},
		AddonsTemplateURL:        o.addonsURL,
		AdditionalTags:           tags.Merge(o.targetApp.Tags, o.resourceTags),
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                o.targetEnvironment.AccountID,
		Region:                   o.targetEnvironment.Region,
	}, nil
}

func (o *deployJobOpts) manifest() (interface{}, error) {
	if o.appliedManifest != nil {
		return o.appliedManifest, nil
	}
	raw, err := o.ws.ReadWorkloadManifest(o.name)
	if err != nil {
		return nil, fmt.Errorf("read job %s manifest: %w", o.name, err)
	}
	interpolated, err := o.newInterpolator(o.appName, o.envName).Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", o.name, err)
	}
	mft, err := o.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal job %s manifest: %w", o.name, err)
	}
	envMft, err := mft.ApplyEnv(o.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", o.envName, err)
	}
	if err := envMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest against environment %s: %s", o.envName, err)
	}
	o.appliedManifest = envMft // cache the results.
	return envMft, nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deployJobOpts) RecommendActions() error {
	return nil
}

func (o *deployJobOpts) validateJobName() error {
	names, err := o.ws.ListJobs()
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

// buildJobDeployCmd builds the `job deploy` subcommand.
func buildJobDeployCmd() *cobra.Command {
	vars := deployWkldVars{}
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
