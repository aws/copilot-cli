// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

type deployJobOpts struct {
	deployWkldVars

	store              store
	ws                 wsJobDirReader
	fs                 *afero.Afero
	unmarshal          func(in []byte) (manifest.WorkloadManifest, error)
	newInterpolator    func(app, env string) interpolator
	cmd                runner
	addons             templater
	jobCFN             cloudformation.CloudFormation
	imageBuilderPusher imageBuilderPusher
	sessProvider       sessionProvider
	s3                 uploader
	envUpgradeCmd      actionCommand
	endpointGetter     endpointGetter
	jobDeployer        workloadDeployer

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	appTags      map[string]string
	imageRepoURL string
	rootUserARN  string
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
		fs:              &afero.Afero{Fs: afero.NewOsFs()},
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
	if !o.clientConfigured {
		if err := o.configureClients(); err != nil {
			return err
		}
	}
	if err := o.envUpgradeCmd.Execute(); err != nil {
		return fmt.Errorf(`execute "env upgrade --app %s --name %s": %v`, o.appName, o.envName, err)
	}
	uploadOut, err := o.jobDeployer.UploadArtifacts(&clideploy.UploadArtifactsInput{
		FS:                 o.fs,
		Uploader:           o.s3,
		Templater:          o.addons,
		ImageBuilderPusher: o.imageBuilderPusher,
	})
	if err != nil {
		return fmt.Errorf("upload deploy resources for job %s: %w", o.name, err)
	}
	if _, err = o.jobDeployer.DeployWorkload(&clideploy.DeployWorkloadInput{
		ImageDigest:     uploadOut.ImageDigest,
		EnvFileARN:      uploadOut.EnvFileARN,
		AddonsURL:       uploadOut.AddonsURL,
		RootUserARN:     o.rootUserARN,
		Tags:            tags.Merge(o.appTags, o.resourceTags),
		ImageRepoURL:    o.imageRepoURL,
		ForceNewUpdate:  o.forceNewUpdate,
		S3Uploader:      o.s3,
		ServiceDeployer: o.jobCFN,
		EndpointGetter:  o.endpointGetter,
		Spinner:         o.spinner,
		Now:             time.Now,
	}); err != nil {
		return fmt.Errorf("deploy job %s to environment %s: %w", o.name, o.envName, err)
	}

	return nil
}

func (o *deployJobOpts) configureClients() error {
	o.imageTag = imageTagFromGit(o.cmd, o.imageTag) // Best effort assign git tag.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return err
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}
	o.appTags = app.Tags

	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(env.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", env.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
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

	o.s3 = s3.New(envSession)

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

	cmd, err := newEnvUpgradeOpts(envUpgradeVars{
		appName: o.appName,
		name:    env.Name,
	})
	if err != nil {
		return fmt.Errorf("new env upgrade command: %v", err)
	}
	o.envUpgradeCmd = cmd

	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	resources, err := cloudformation.New(defaultSess).GetAppResourcesByRegion(app, env.Region)
	if err != nil {
		return fmt.Errorf("get application %s resources from region %s: %w", app.Name, env.Region, err)
	}
	o.imageRepoURL = resources.RepositoryURLs[o.name]

	deployer, err := clideploy.NewWorkloadDeployer(&clideploy.WorkloadDeployerInput{
		Name:     o.name,
		App:      app,
		Env:      env,
		ImageTag: o.imageTag,
		S3Bucket: resources.S3Bucket,
	})
	if err != nil {
		return fmt.Errorf("initiate workload deployer: %w", err)
	}
	o.jobDeployer = deployer

	return nil
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
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
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
