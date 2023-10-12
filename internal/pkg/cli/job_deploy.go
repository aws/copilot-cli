// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/afero"

	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/spf13/cobra"

	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

type deployJobOpts struct {
	deployWkldVars

	store                store
	ws                   wsWlDirReader
	unmarshal            func(in []byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
	cmd                  execRunner
	jobVersionGetter     versionGetter
	sessProvider         *sessions.Provider
	newJobDeployer       func() (workloadDeployer, error)
	envFeaturesDescriber versionCompatibilityChecker
	sel                  wsSelector
	prompt               prompter
	gitShortCommit       string
	diffWriter           io.Writer

	// cached variables
	targetApp         *config.Application
	targetEnv         *config.Environment
	envSess           *session.Session
	rawMft            string // Content of the environment manifest with env var interpolation only.
	appliedDynamicMft manifest.DynamicWorkload
	rootUserARN       string

	// Overridden in tests.
	templateVersion string
}

func newJobDeployOpts(vars deployWkldVars) (*deployJobOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("job deploy"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	prompter := prompt.New()
	opts := &deployJobOpts{
		deployWkldVars: vars,

		store:           store,
		ws:              ws,
		unmarshal:       manifest.UnmarshalWorkload,
		sel:             selector.NewLocalWorkloadSelector(prompter, store, ws, selector.OnlyInitializedWorkloads),
		prompt:          prompter,
		sessProvider:    sessProvider,
		newInterpolator: newManifestInterpolator,
		cmd:             exec.NewCmd(),
		templateVersion: version.LatestTemplateVersion(),
		diffWriter:      os.Stdout,
	}
	opts.newJobDeployer = func() (workloadDeployer, error) {
		// NOTE: Defined as a struct member to facilitate unit testing.
		return newJobDeployer(opts)
	}
	return opts, nil
}

func newJobDeployer(o *deployJobOpts) (workloadDeployer, error) {
	ovrdr, err := deploy.NewOverrider(o.ws.WorkloadOverridesPath(o.name), o.appName, o.envName, afero.NewOsFs(), o.sessProvider)
	if err != nil {
		return nil, err
	}

	content := o.appliedDynamicMft.Manifest()
	in := deploy.WorkloadDeployerInput{
		SessionProvider: o.sessProvider,
		Name:            o.name,
		App:             o.targetApp,
		Env:             o.targetEnv,
		Image: deploy.ContainerImageIdentifier{
			CustomTag:         o.imageTag,
			GitShortCommitTag: o.gitShortCommit,
		},
		Mft:              content,
		RawMft:           o.rawMft,
		EnvVersionGetter: o.envFeaturesDescriber,
		Overrider:        ovrdr,
	}
	var deployer workloadDeployer
	switch t := content.(type) {
	case *manifest.ScheduledJob:
		deployer, err = deploy.NewJobDeployer(&in)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("initiate workload deployer: %w", err)
	}
	return deployer, nil
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
	if !o.allowWkldDowngrade {
		if err := validateWkldVersion(o.jobVersionGetter, o.name, o.templateVersion); err != nil {
			return err
		}
	}
	mft, interpolated, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		ws:           o.ws,
		interpolator: o.newInterpolator(o.appName, o.envName),
		unmarshal:    o.unmarshal,
		sess:         o.envSess,
	})
	if err != nil {
		return err
	}
	o.appliedDynamicMft = mft
	o.rawMft = interpolated
	if err := validateWorkloadManifestCompatibilityWithEnv(o.ws, o.envFeaturesDescriber, mft, o.envName); err != nil {
		return err
	}
	deployer, err := o.newJobDeployer()
	if err != nil {
		return err
	}
	serviceInRegion, err := deployer.IsServiceAvailableInRegion(o.targetEnv.Region)
	if err != nil {
		return fmt.Errorf("check if Scheduled Job(s) is available in region %s: %w", o.targetEnv.Region, err)
	}

	if !serviceInRegion {
		log.Warningf(`Scheduled Job might not be available in region %s; proceed with caution.
`, o.targetEnv.Region)
	}
	uploadOut, err := deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload deploy resources for job %s: %w", o.name, err)
	}
	if o.showDiff {
		output, err := deployer.GenerateCloudFormationTemplate(&deploy.GenerateCloudFormationTemplateInput{
			StackRuntimeConfiguration: deploy.StackRuntimeConfiguration{
				RootUserARN:        o.rootUserARN,
				Tags:               o.targetApp.Tags,
				EnvFileARNs:        uploadOut.EnvFileARNs,
				ImageDigests:       uploadOut.ImageDigests,
				AddonsURL:          uploadOut.AddonsURL,
				Version:            o.templateVersion,
				CustomResourceURLs: uploadOut.CustomResourceURLs,
			},
		})
		if err != nil {
			return fmt.Errorf("generate the template for job %q against environment %q: %w", o.name, o.envName, err)
		}
		if err := diff(deployer, output.Template, o.diffWriter); err != nil {
			var errHasDiff *errHasDiff
			if !errors.As(err, &errHasDiff) {
				return err
			}
		}
		contd, err := o.prompt.Confirm(continueDeploymentPrompt, "")
		if err != nil {
			return fmt.Errorf("ask whether to continue with the deployment: %w", err)
		}
		if !contd {
			return nil
		}
	}
	if _, err = deployer.DeployWorkload(&deploy.DeployWorkloadInput{
		StackRuntimeConfiguration: deploy.StackRuntimeConfiguration{
			ImageDigests:       uploadOut.ImageDigests,
			EnvFileARNs:        uploadOut.EnvFileARNs,
			AddonsURL:          uploadOut.AddonsURL,
			RootUserARN:        o.rootUserARN,
			Version:            o.templateVersion,
			Tags:               tags.Merge(o.targetApp.Tags, o.resourceTags),
			CustomResourceURLs: uploadOut.CustomResourceURLs,
		},
		Options: deploy.Options{
			DisableRollback: o.disableRollback,
			Detach:          o.detach,
		},
	}); err != nil {
		var errStackDeletedOnInterrupt *deploycfn.ErrStackDeletedOnInterrupt
		var errStackUpdateCanceledOnInterrupt *deploycfn.ErrStackUpdateCanceledOnInterrupt
		var errEmptyChangeSet *awscfn.ErrChangeSetEmpty
		if errors.As(err, &errStackDeletedOnInterrupt) {
			return nil
		}
		if errors.As(err, &errStackUpdateCanceledOnInterrupt) {
			log.Successf("Successfully rolled back service %s to the previous configuration.\n", color.HighlightUserInput(o.name))
			return nil
		}
		if o.disableRollback {
			stackName := stack.NameForWorkload(o.targetApp.Name, o.targetEnv.Name, o.name)
			rollbackCmd := fmt.Sprintf("aws cloudformation rollback-stack --stack-name %s --role-arn %s", stackName, o.targetEnv.ExecutionRoleARN)
			log.Infof(`It seems like you have disabled automatic stack rollback for this deployment. To debug, you can visit the AWS console to inspect the errors.
After fixing the deployment, you can:
1. Run %s to rollback the deployment.
2. Run %s to make a new deployment.
`, color.HighlightCode(rollbackCmd), color.HighlightCode("copilot job deploy"))
		}
		if errors.As(err, &errEmptyChangeSet) {
			return &errNoInfrastructureChanges{parentErr: err}
		}
		return fmt.Errorf("deploy job %s to environment %s: %w", o.name, o.envName, err)
	}
	if o.detach {
		return nil
	}
	log.Successf("Deployed %s.\n", color.HighlightUserInput(o.name))
	return nil
}

func (o *deployJobOpts) configureClients() error {
	o.gitShortCommit = imageTagFromGit(o.cmd) // Best effort assign git tag.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return err
	}
	o.targetEnv = env
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}
	o.targetApp = app

	// client to retrieve an application's resources created with CloudFormation
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	envSess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return err
	}
	o.envSess = envSess

	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return err
	}
	o.envFeaturesDescriber = envDescriber

	wkldDescriber, err := describe.NewWorkloadStackDescriber(describe.NewWorkloadConfig{
		App:         o.appName,
		Env:         o.envName,
		Name:        o.name,
		ConfigStore: o.store,
	})
	if err != nil {
		return err
	}
	o.jobVersionGetter = wkldDescriber
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
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.showDiff, diffFlag, false, diffFlagDescription)
	cmd.Flags().BoolVar(&vars.allowWkldDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.detach, detachFlag, false, detachFlagDescription)
	return cmd
}
