// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

type deployWkldVars struct {
	appName         string
	name            string
	envName         string
	imageTag        string
	resourceTags    map[string]string
	forceNewUpdate  bool // NOTE: this variable is not applicable for a job workload currently.
	disableRollback bool

	// To facilitate unit tests.
	clientConfigured bool
}

type deploySvcOpts struct {
	deployWkldVars

	store           store
	ws              wsWlDirReader
	unmarshal       func([]byte) (manifest.WorkloadManifest, error)
	newInterpolator func(app, env string) interpolator
	cmd             runner
	envUpgradeCmd   actionCommand
	sessProvider    *sessions.Provider
	newSvcDeployer  func() (workloadDeployer, error)

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	targetApp       *config.Application
	targetEnv       *config.Environment
	svcType         string
	appliedManifest interface{}
	rootUserARN     string
	deployRecs      deploy.ActionRecommender
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc deploy"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	opts := &deploySvcOpts{
		deployWkldVars: vars,

		store:           store,
		ws:              ws,
		unmarshal:       manifest.UnmarshalWorkload,
		spinner:         termprogress.NewSpinner(log.DiagnosticWriter),
		sel:             selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:          prompter,
		newInterpolator: newManifestInterpolator,
		cmd:             exec.NewCmd(),
		sessProvider:    sessProvider,
	}
	opts.newSvcDeployer = func() (workloadDeployer, error) {
		// NOTE: Defined as a struct member to facilitate unit testing.
		return newSvcDeployer(opts)
	}
	return opts, err
}

func newSvcDeployer(o *deploySvcOpts) (workloadDeployer, error) {
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
	}
	var deployer workloadDeployer
	in := deploy.WorkloadDeployerInput{
		SessionProvider: o.sessProvider,
		Name:            o.name,
		App:             targetApp,
		Env:             o.targetEnv,
		ImageTag:        o.imageTag,
		Mft:             o.appliedManifest,
	}
	switch t := o.appliedManifest.(type) {
	case *manifest.LoadBalancedWebService:
		deployer, err = deploy.NewLBDeployer(&in)
	case *manifest.BackendService:
		deployer, err = deploy.NewBackendDeployer(&in)
	case *manifest.RequestDrivenWebService:
		deployer, err = deploy.NewRDWSDeployer(&in)
	case *manifest.WorkerService:
		deployer, err = deploy.NewWorkerSvcDeployer(&in)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("initiate workload deployer: %w", err)
	}
	return deployer, nil
}

func newManifestInterpolator(app, env string) interpolator {
	return manifest.NewInterpolator(app, env)
}

// Validate returns an error for any invalid optional flags.
func (o *deploySvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *deploySvcOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.getTargetApp(); err != nil {
			return err
		}
	} else {
		// NOTE: This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}

	if err := o.validateOrAskSvcName(); err != nil {
		return err
	}

	if err := o.validateOrAskEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the service,
func (o *deploySvcOpts) Execute() error {
	if !o.clientConfigured {
		if err := o.configureClients(); err != nil {
			return err
		}
	}
	if err := o.envUpgradeCmd.Execute(); err != nil {
		return fmt.Errorf(`execute "env upgrade --app %s --name %s": %v`, o.appName, o.envName, err)
	}
	mft, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		interpolator: o.newInterpolator(o.appName, o.envName),
		ws:           o.ws,
		unmarshal:    o.unmarshal,
	})
	if err != nil {
		return err
	}
	o.appliedManifest = mft
	deployer, err := o.newSvcDeployer()
	if err != nil {
		return err
	}
	serviceInRegion, err := deployer.IsServiceAvailableInRegion(o.targetEnv.Region)
	if err != nil {
		return fmt.Errorf("check if %s is available in region %s: %w", o.svcType, o.targetEnv.Region, err)
	}

	if !serviceInRegion {
		log.Warningf(`%s might not be available in region %s; proceed with caution.
`, o.svcType, o.targetEnv.Region)
	}
	uploadOut, err := deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload deploy resources for service %s: %w", o.name, err)
	}
	targetApp, err := o.getTargetApp()
	if err != nil {
		return err
	}
	deployRecs, err := deployer.DeployWorkload(&deploy.DeployWorkloadInput{
		StackRuntimeConfiguration: deploy.StackRuntimeConfiguration{
			ImageDigest: uploadOut.ImageDigest,
			EnvFileARN:  uploadOut.EnvFileARN,
			AddonsURL:   uploadOut.AddonsURL,
			RootUserARN: o.rootUserARN,
			Tags:        tags.Merge(targetApp.Tags, o.resourceTags),
		},
		Options: deploy.Options{
			ForceNewUpdate:  o.forceNewUpdate,
			DisableRollback: o.disableRollback,
		},
	})
	if err != nil {
		if o.disableRollback {
			stackName := stack.NameForService(o.targetApp.Name, o.targetEnv.Name, o.name)
			rollbackCmd := fmt.Sprintf("aws cloudformation rollback-stack --stack-name %s --role-arn %s", stackName, o.targetEnv.ExecutionRoleARN)
			log.Infof(`It seems like you have disabled automatic stack rollback for this deployment. To debug, you can:
* Run %s to inspect the service log.
* Visit the AWS console to inspect the errors.
After fixing the deployment, you can:
1. Run %s to rollback the deployment.
2. Run %s to make a new deployment.
`, color.HighlightCode("copilot svc logs"), color.HighlightCode(rollbackCmd), color.HighlightCode("copilot svc deploy"))
		}
		return fmt.Errorf("deploy service %s to environment %s: %w", o.name, o.envName, err)
	}
	o.deployRecs = deployRecs
	log.Successf("Deployed service %s.\n", color.HighlightUserInput(o.name))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendActions() error {
	var recommendations []string
	uriRecs, err := o.uriRecommendedActions()
	if err != nil {
		return err
	}
	recommendations = append(recommendations, uriRecs...)
	if o.deployRecs != nil {
		recommendations = append(recommendations, o.deployRecs.RecommendedActions()...)
	}
	recommendations = append(recommendations, o.publishRecommendedActions()...)
	logRecommendedActions(recommendations)
	return nil
}

func (o *deploySvcOpts) validateSvcName() error {
	names, err := o.ws.ListServices()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %w", err)
	}
	for _, name := range names {
		if o.name == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", color.HighlightUserInput(o.name))
}

func (o *deploySvcOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	return nil
}

func (o *deploySvcOpts) validateOrAskSvcName() error {
	if o.name != "" {
		return o.validateSvcName()
	}

	name, err := o.sel.Service("Select a service in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *deploySvcOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		return o.validateEnvName()
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deploySvcOpts) configureClients() error {
	o.imageTag = imageTagFromGit(o.cmd, o.imageTag) // Best effort assign git tag.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	o.targetEnv = env
	svc, err := o.store.GetService(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get service %s configuration: %w", o.name, err)
	}
	o.svcType = svc.Type

	cmd, err := newEnvUpgradeOpts(envUpgradeVars{
		appName: o.appName,
		name:    env.Name,
	})
	if err != nil {
		return fmt.Errorf("new env upgrade command: %v", err)
	}
	o.envUpgradeCmd = cmd

	// client to retrieve an application's resources created with CloudFormation.
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}

	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	return nil
}

type workloadManifestInput struct {
	name         string
	appName      string
	envName      string
	ws           wsWlDirReader
	interpolator interpolator
	unmarshal    func([]byte) (manifest.WorkloadManifest, error)
}

func workloadManifest(in *workloadManifestInput) (interface{}, error) {
	raw, err := in.ws.ReadWorkloadManifest(in.name)
	if err != nil {
		return nil, fmt.Errorf("read manifest file for %s: %w", in.name, err)
	}
	interpolated, err := in.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", in.name, err)
	}
	mft, err := in.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", in.name, err)
	}
	envMft, err := mft.ApplyEnv(in.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", in.envName, err)
	}

	if err := envMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest against environment %s: %s", in.envName, err)
	}
	return envMft, nil
}

func (o *deploySvcOpts) uriRecommendedActions() ([]string, error) {
	type reachable interface {
		Port() (uint16, bool)
	}
	mft, ok := o.appliedManifest.(reachable)
	if !ok {
		return nil, nil
	}
	if _, ok := mft.Port(); !ok { // No exposed port.
		return nil, nil
	}

	describer, err := describe.NewReachableService(o.appName, o.name, o.store)
	if err != nil {
		return nil, err
	}
	uri, err := describer.URI(o.envName)
	if err != nil {
		return nil, fmt.Errorf("get uri for environment %s: %w", o.envName, err)
	}

	network := "over the internet."
	if o.svcType == manifest.BackendServiceType {
		network = "with service discovery."
	}
	recs := []string{
		fmt.Sprintf("You can access your service at %s %s", color.HighlightResource(uri), network),
	}
	return recs, nil
}

func (o *deploySvcOpts) publishRecommendedActions() []string {
	type publisher interface {
		Publish() []manifest.Topic
	}
	mft, ok := o.appliedManifest.(publisher)
	if !ok {
		return nil
	}
	if topics := mft.Publish(); len(topics) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf(`Update %s's code to leverage the injected environment variable "COPILOT_SNS_TOPIC_ARNS".
    In JavaScript you can write %s.`,
			o.name,
			color.HighlightCode("const {<topicName>} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)")),
	}
}

func (o *deploySvcOpts) getTargetApp() (*config.Application, error) {
	if o.targetApp != nil {
		return o.targetApp, nil
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s configuration: %w", o.appName, err)
	}
	o.targetApp = app
	return o.targetApp, nil
}

// buildSvcDeployCmd builds the `svc deploy` subcommand.
func buildSvcDeployCmd() *cobra.Command {
	vars := deployWkldVars{}
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
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)

	return cmd
}
