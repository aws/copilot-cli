// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	cs "github.com/aws/copilot-cli/internal/pkg/aws/codestar"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/list"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	templatediff "github.com/aws/copilot-cli/internal/pkg/template/diff"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/spf13/cobra"

	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
)

const (
	pipelineSelectPrompt = "Select a pipeline from your workspace to deploy"

	fmtPipelineDeployResourcesStart    = "Adding pipeline resources to your application: %s"
	fmtPipelineDeployResourcesFailed   = "Failed to add pipeline resources to your application: %s\n"
	fmtPipelineDeployResourcesComplete = "Successfully added pipeline resources to your application: %s\n"

	fmtPipelineDeployStart    = "Creating a new pipeline: %s"
	fmtPipelineDeployFailed   = "Failed to create a new pipeline: %s.\n"
	fmtPipelineDeployComplete = "Successfully created a new pipeline: %s\n"

	fmtPipelineDeployProposalStart    = "Proposing infrastructure changes for the pipeline: %s"
	fmtPipelineDeployProposalFailed   = "Failed to accept changes for pipeline: %s.\n"
	fmtPipelineDeployProposalComplete = "Successfully deployed pipeline: %s\n"

	fmtPipelineDeployExistPrompt = "Are you sure you want to redeploy an existing pipeline: %s?"
)

const connectionsURL = "https://console.aws.amazon.com/codesuite/settings/connections"

type deployPipelineVars struct {
	appName          string
	name             string
	skipConfirmation bool
	showDiff         bool
	allowDowngrade   bool
}

type newOverrideOpts struct {
	path       string
	appName    string
	envName    string
	fileSystem afero.Fs
	sess       *sessions.Provider
}

type deployPipelineOpts struct {
	deployPipelineVars

	pipelineDeployer      pipelineDeployer
	sel                   wsPipelineSelector
	prog                  progress
	prompt                prompter
	region                string
	store                 store
	ws                    wsPipelineReader
	codestar              codestar
	diffWriter            io.Writer
	sessProvider          *sessions.Provider
	newSvcListCmd         func(io.Writer, string) cmd
	newJobListCmd         func(io.Writer, string) cmd
	pipelineVersionGetter func(string, string, bool) (versionGetter, error)
	pipelineStackConfig   func(in *deploy.CreatePipelineInput) stackConfiguration

	configureDeployedPipelineLister func() deployedPipelineLister

	// cached variables
	wsAppName                    string
	app                          *config.Application
	pipeline                     *workspace.PipelineManifest
	shouldPromptUpdateConnection bool
	isLegacyPipeline             *bool
	pipelineMft                  *manifest.Pipeline
	svcBuffer                    *bytes.Buffer
	jobBuffer                    *bytes.Buffer

	// Overridden in tests.
	templateVersion string
}

func newDeployPipelineOpts(vars deployPipelineVars) (*deployPipelineOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline deploy"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))

	prompter := prompt.New()
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	wsAppName := tryReadingAppName()
	if vars.appName == "" {
		vars.appName = wsAppName
	}

	opts := &deployPipelineOpts{
		ws:                 ws,
		pipelineDeployer:   deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		region:             aws.StringValue(defaultSession.Config.Region),
		deployPipelineVars: vars,
		store:              store,
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:             prompter,
		diffWriter:         os.Stdout,
		sessProvider:       sessProvider,
		sel:                selector.NewWsPipelineSelector(prompter, ws),
		codestar:           cs.New(defaultSession),
		templateVersion:    version.LatestTemplateVersion(),
		pipelineStackConfig: func(in *deploy.CreatePipelineInput) stackConfiguration {
			return stack.NewPipelineStackConfig(in)
		},
		newSvcListCmd: func(w io.Writer, appName string) cmd {
			return &listSvcOpts{
				listWkldVars: listWkldVars{
					appName: appName,
				},
				sel: selector.NewAppEnvSelector(prompt.New(), store),
				list: &list.SvcListWriter{
					Ws:    ws,
					Store: store,
					Out:   w,

					ShowLocalSvcs: true,
					OutputJSON:    true,
				},
			}
		},
		newJobListCmd: func(w io.Writer, appName string) cmd {
			return &listJobOpts{
				listWkldVars: listWkldVars{
					appName: appName,
				},
				sel: selector.NewAppEnvSelector(prompt.New(), store),
				list: &list.JobListWriter{
					Ws:    ws,
					Store: store,
					Out:   w,

					ShowLocalJobs: true,
					OutputJSON:    true,
				},
			}
		},
		wsAppName: wsAppName,
		svcBuffer: &bytes.Buffer{},
		jobBuffer: &bytes.Buffer{},
	}
	opts.configureDeployedPipelineLister = func() deployedPipelineLister {
		// Initialize the client only after the appName is asked.
		return deploy.NewPipelineStore(rg.New(defaultSession))
	}
	opts.pipelineVersionGetter = func(appName, name string, isLegacy bool) (versionGetter, error) {
		return describe.NewPipelineStackDescriber(appName, name, isLegacy)
	}
	return opts, nil
}

// Validate returns an error if the optional flag values passed by the user are invalid.
func (o *deployPipelineOpts) Validate() error {
	return nil
}

// Ask prompts the user for any unprovided required fields and validates them.
func (o *deployPipelineOpts) Ask() error {
	if o.wsAppName == "" {
		return errNoAppInWorkspace
	}
	// This command must be run within the app's workspace.
	if o.appName != "" && o.appName != o.wsAppName {
		return fmt.Errorf("cannot specify app %s because the workspace is already registered with app %s", o.appName, o.wsAppName)
	}
	appConfig, err := o.store.GetApplication(o.wsAppName)
	if err != nil {
		return fmt.Errorf("get application %s configuration: %w", o.wsAppName, err)
	}
	o.app = appConfig

	if o.name != "" {
		return o.validatePipelineName()
	}

	return o.askWsPipelineName()
}

func validatePipelineVersion(vg versionGetter, name, templateVersion string) error {
	pipelineVersion, err := vg.Version()
	if err != nil {
		var errStackNotExist *cloudformation.ErrStackNotFound
		if errors.As(err, &errStackNotExist) {
			return nil
		}
		return fmt.Errorf("get template version of pipeline %s: %w", name, err)
	}
	if semver.Compare(pipelineVersion, templateVersion) > 0 {
		return &errCannotDowngradePipelineVersion{
			name:            name,
			version:         pipelineVersion,
			templateVersion: templateVersion,
		}
	}
	return nil
}

// Execute creates a new pipeline or updates the current pipeline if it already exists.
func (o *deployPipelineOpts) Execute() error {
	if !o.allowDowngrade {
		isLegacy, err := o.isLegacy(o.name)
		if err != nil {
			return err
		}
		pipelineVersionGetter, err := o.pipelineVersionGetter(o.appName, o.name, isLegacy)
		if err != nil {
			return err
		}
		if err := validatePipelineVersion(pipelineVersionGetter, o.name, o.templateVersion); err != nil {
			return err
		}
	}

	// Read pipeline manifest.
	pipeline, err := o.getPipelineMft()
	if err != nil {
		return err
	}

	// If the source has an existing connection, get the correlating ConnectionARN.
	connection, ok := pipeline.Source.Properties["connection_name"]
	if ok {
		arn, err := o.codestar.GetConnectionARN((connection).(string))
		if err != nil {
			return fmt.Errorf("get connection ARN: %w", err)
		}
		pipeline.Source.Properties["connection_arn"] = arn
	}

	source, shouldPrompt, err := deploy.PipelineSourceFromManifest(pipeline.Source)
	if err != nil {
		return fmt.Errorf("read source from manifest: %w", err)
	}
	o.shouldPromptUpdateConnection = shouldPrompt

	// Convert full manifest path to relative path from workspace root.
	relPath, err := o.ws.Rel(o.pipeline.Path)
	if err != nil {
		return err
	}

	// Convert environments to deployment stages.
	stages, err := o.convertStages(pipeline.Stages)
	if err != nil {
		return fmt.Errorf("convert environments to deployment stage: %w", err)
	}

	// Get cross-regional resources.
	artifactBuckets, err := o.getArtifactBuckets()
	if err != nil {
		return fmt.Errorf("get cross-regional resources: %w", err)
	}

	isLegacy, err := o.isLegacy(pipeline.Name)
	if err != nil {
		return err
	}
	var build deploy.Build
	if err = build.Init(pipeline.Build, filepath.Dir(relPath)); err != nil {
		return err
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		AppName:             o.appName,
		Name:                pipeline.Name,
		IsLegacy:            isLegacy,
		Source:              source,
		Build:               &build,
		Stages:              stages,
		ArtifactBuckets:     artifactBuckets,
		AdditionalTags:      o.app.Tags,
		Version:             o.templateVersion,
		PermissionsBoundary: o.app.PermissionsBoundary,
	}

	overrideOpts := newOverrideOpts{
		path:       o.ws.PipelineOverridesPath(o.pipeline.Name),
		appName:    o.appName,
		fileSystem: afero.NewOsFs(),
		sess:       o.sessProvider,
	}

	overrider, err := clideploy.NewOverrider(overrideOpts.path, overrideOpts.appName, overrideOpts.envName, overrideOpts.fileSystem, overrideOpts.sess)
	if err != nil {
		return err
	}
	stackConfig := deploycfn.WrapWithTemplateOverrider(o.pipelineStackConfig(deployPipelineInput), overrider)

	if o.showDiff {
		tpl, err := stackConfig.Template()
		if err != nil {
			return fmt.Errorf("generate the new template for diff: %w", err)
		}
		if err = diff(o, tpl, o.diffWriter); err != nil {
			var errHasDiff *errHasDiff
			if !errors.As(err, &errHasDiff) {
				return err
			}
		}
		if !o.skipConfirmation {
			contd, err := o.prompt.Confirm(continueDeploymentPrompt, "")
			if err != nil {
				return fmt.Errorf("ask whether to continue with the deployment: %w", err)
			}
			if !contd {
				return nil
			}
		}
	}

	// bootstrap pipeline resources
	o.prog.Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, color.HighlightUserInput(o.appName)))
	err = o.pipelineDeployer.AddPipelineResourcesToApp(o.app, o.region)
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtPipelineDeployResourcesFailed, color.HighlightUserInput(o.appName)))
		return fmt.Errorf("add pipeline resources to application %s in %s: %w", o.appName, o.region, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, color.HighlightUserInput(o.appName)))

	if err := o.deployPipeline(deployPipelineInput, stackConfig); err != nil {
		return err
	}
	return nil
}

// DeployDiff returns the stringified diff of the template against the deployed template of the pipeline.
func (o *deployPipelineOpts) DeployDiff(template string) (string, error) {
	isLegacy, err := o.isLegacy(o.pipeline.Name)
	if err != nil {
		return "", err
	}

	tmpl, err := o.pipelineDeployer.Template(stack.NameForPipeline(o.app.Name, o.pipeline.Name, isLegacy))
	if err != nil {
		var errNotFound *awscloudformation.ErrStackNotFound
		if !errors.As(err, &errNotFound) {
			return "", fmt.Errorf("retrieve the deployed template for %q: %w", o.pipeline.Name, err)
		}
		tmpl = ""
	}
	diffTree, err := templatediff.From(tmpl).ParseWithCFNOverriders([]byte(template))
	if err != nil {
		return "", fmt.Errorf("parse the diff against the deployed pipeline stack %q: %w", o.pipeline.Name, err)
	}
	buf := strings.Builder{}
	if err := diffTree.Write(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (o *deployPipelineOpts) isLegacy(inputName string) (bool, error) {
	if o.isLegacyPipeline != nil {
		return *o.isLegacyPipeline, nil
	}
	lister := o.configureDeployedPipelineLister()
	pipelines, err := lister.ListDeployedPipelines(o.appName)
	if err != nil {
		o.isLegacyPipeline = aws.Bool(false)
		return false, fmt.Errorf("list deployed pipelines for app %s: %w", o.appName, err)
	}
	for _, pipeline := range pipelines {
		if pipeline.ResourceName == inputName {
			// NOTE: this is double insurance. A namespaced pipeline's `ResourceName` wouldn't be equal to
			// `inputName` in the first place, because it would have been namespaced and have random string
			// appended by CFN.
			o.isLegacyPipeline = aws.Bool(pipeline.IsLegacy)
			return pipeline.IsLegacy, nil
		}
	}
	o.isLegacyPipeline = aws.Bool(false)
	return false, nil
}

func (o *deployPipelineOpts) validatePipelineName() error {
	pipelines, err := o.ws.ListPipelines()
	if err != nil {
		return fmt.Errorf("list pipelines: %w", err)
	}
	for _, pipeline := range pipelines {
		if pipeline.Name == o.name {
			o.pipeline = &pipeline
			return nil
		}
	}
	return fmt.Errorf(`pipeline %s not found in the workspace`, color.HighlightUserInput(o.name))
}

func (o *deployPipelineOpts) askWsPipelineName() error {
	pipeline, err := o.sel.WsPipeline(pipelineSelectPrompt, "")
	if err != nil {
		return fmt.Errorf("select pipeline: %w", err)
	}
	o.pipeline = pipeline
	o.name = pipeline.Name

	return nil
}

func (o *deployPipelineOpts) getPipelineMft() (*manifest.Pipeline, error) {
	if o.pipelineMft != nil {
		return o.pipelineMft, nil
	}

	pipelineMft, err := o.ws.ReadPipelineManifest(o.pipeline.Path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline manifest: %w", err)
	}

	if err := pipelineMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate pipeline manifest: %w", err)
	}
	o.pipelineMft = pipelineMft
	return pipelineMft, nil
}

func (o *deployPipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
	var stages []deploy.PipelineStage
	workloads, err := o.getLocalWorkloads()
	if err != nil {
		return nil, err
	}
	for _, stage := range manifestStages {
		env, err := o.store.GetEnvironment(o.appName, stage.Name)
		if err != nil {
			return nil, fmt.Errorf("get environment %s in application %s: %w", stage.Name, o.appName, err)
		}

		var stg deploy.PipelineStage
		stg.Init(env, &stage, workloads)
		stages = append(stages, stg)
	}
	return stages, nil
}

func (o deployPipelineOpts) getLocalWorkloads() ([]string, error) {
	var localWklds []string
	if err := o.newSvcListCmd(o.svcBuffer, o.appName).Execute(); err != nil {
		return nil, fmt.Errorf("get local services: %w", err)
	}
	if err := o.newJobListCmd(o.jobBuffer, o.appName).Execute(); err != nil {
		return nil, fmt.Errorf("get local jobs: %w", err)
	}
	svcOutput, jobOutput := &list.ServiceJSONOutput{}, &list.JobJSONOutput{}
	if err := json.Unmarshal(o.svcBuffer.Bytes(), svcOutput); err != nil {
		return nil, fmt.Errorf("unmarshal service list output; %w", err)
	}
	for _, svc := range svcOutput.Services {
		localWklds = append(localWklds, svc.Name)
	}
	if err := json.Unmarshal(o.jobBuffer.Bytes(), jobOutput); err != nil {
		return nil, fmt.Errorf("unmarshal job list output; %w", err)
	}
	for _, job := range jobOutput.Jobs {
		localWklds = append(localWklds, job.Name)
	}
	return localWklds, nil
}

func (o *deployPipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
	regionalResources, err := o.pipelineDeployer.GetRegionalAppResources(o.app)
	if err != nil {
		return nil, err
	}

	var buckets []deploy.ArtifactBucket
	for _, resource := range regionalResources {
		bucket := deploy.ArtifactBucket{
			BucketName: resource.S3Bucket,
			KeyArn:     resource.KMSKeyARN,
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

func (o *deployPipelineOpts) getBucketName() (string, error) {
	resources, err := o.pipelineDeployer.GetAppResourcesByRegion(o.app, o.region)
	if err != nil {
		return "", fmt.Errorf("get app resources: %w", err)
	}
	return resources.S3Bucket, nil
}

func (o *deployPipelineOpts) shouldUpdate() (bool, error) {
	if o.skipConfirmation {
		return true, nil
	}

	shouldUpdate, err := o.prompt.Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, o.pipeline.Name), "")
	if err != nil {
		return false, fmt.Errorf("prompt for pipeline deploy: %w", err)
	}
	return shouldUpdate, nil
}

func (o *deployPipelineOpts) deployPipeline(in *deploy.CreatePipelineInput, stackConfig deploycfn.StackConfiguration) error {
	exist, err := o.pipelineDeployer.PipelineExists(stackConfig)
	if err != nil {
		return fmt.Errorf("check if pipeline exists: %w", err)
	}

	// Find the bucket to push the pipeline template to.
	bucketName, err := o.getBucketName()
	if err != nil {
		return fmt.Errorf("get bucket name: %w", err)
	}
	if !exist {
		o.prog.Start(fmt.Sprintf(fmtPipelineDeployStart, color.HighlightUserInput(o.pipeline.Name)))

		// If the source requires CodeStar Connections, the user is prompted to update the connection status.
		if o.shouldPromptUpdateConnection {
			source, ok := in.Source.(interface {
				ConnectionName() (string, error)
			})
			if !ok {
				return fmt.Errorf("source %v does not have a connection name", in.Source)
			}
			connectionName, err := source.ConnectionName()
			if err != nil {
				return fmt.Errorf("parse connection name: %w", err)
			}
			log.Infoln()
			log.Infof("%s Go to %s to update the status of connection %s from PENDING to AVAILABLE.", color.Emphasize("ACTION REQUIRED!"), color.HighlightResource(connectionsURL), color.HighlightUserInput(connectionName))
			log.Infoln()
		}
		if err := o.pipelineDeployer.CreatePipeline(bucketName, stackConfig); err != nil {
			var alreadyExists *cloudformation.ErrStackAlreadyExists
			if !errors.As(err, &alreadyExists) {
				o.prog.Stop(log.Serrorf(fmtPipelineDeployFailed, color.HighlightUserInput(o.pipeline.Name)))
				return fmt.Errorf("create pipeline: %w", err)
			}
		}
		o.prog.Stop(log.Ssuccessf(fmtPipelineDeployComplete, color.HighlightUserInput(o.pipeline.Name)))
		return nil
	}

	// If the stack already exists - we update it
	if !o.showDiff {
		shouldUpdate, err := o.shouldUpdate()
		if err != nil {
			return err
		}
		if !shouldUpdate {
			return nil
		}
	}

	o.prog.Start(fmt.Sprintf(fmtPipelineDeployProposalStart, color.HighlightUserInput(o.pipeline.Name)))
	if err := o.pipelineDeployer.UpdatePipeline(bucketName, stackConfig); err != nil {
		o.prog.Stop(log.Serrorf(fmtPipelineDeployProposalFailed, color.HighlightUserInput(o.pipeline.Name)))
		return fmt.Errorf("update pipeline: %w", err)
	}
	o.prog.Stop(log.Ssuccessf(fmtPipelineDeployProposalComplete, color.HighlightUserInput(o.pipeline.Name)))
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deployPipelineOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Run %s to see the state of your pipeline.", color.HighlightCode("copilot pipeline status")),
		fmt.Sprintf("Run %s for info about your pipeline.", color.HighlightCode("copilot pipeline show")),
	}
}

// BuildPipelineDeployCmd build the command for deploying a new pipeline or updating an existing pipeline.
func buildPipelineDeployCmd() *cobra.Command {
	vars := deployPipelineVars{}
	cmd := &cobra.Command{
		Use:     "deploy",
		Aliases: []string{"update"},
		Short:   "Deploys a pipeline for the services in your workspace.",
		Long:    `Deploys a pipeline for the services in your workspace, using the environments associated with the application.`,
		Example: `
  Deploys a pipeline for the services and jobs in your workspace.
  /code $ copilot pipeline deploy
`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeployPipelineOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	cmd.Flags().BoolVar(&vars.showDiff, diffFlag, false, diffFlagDescription)
	cmd.Flags().BoolVar(&vars.allowDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	return cmd
}
