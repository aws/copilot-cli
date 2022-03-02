// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	cs "github.com/aws/copilot-cli/internal/pkg/aws/codestar"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/list"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"

	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	pipelineDeployAppNamePrompt     = "Which application's pipeline would you like to deploy?"
	pipelineDeployAppNameHelpPrompt = "An application is a collection of related services."
	pipelineSelectPrompt            = "Select a pipeline from your workspace to deploy"

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
}

type deployPipelineOpts struct {
	deployPipelineVars

	pipelineDeployer pipelineDeployer
	app              *config.Application
	sel              wsPipelineSelector
	prog             progress
	prompt           prompter
	region           string
	store            appEnvStore
	ws               wsPipelineReader
	codestar         codestar
	newSvcListCmd    func(io.Writer) cmd
	newJobListCmd    func(io.Writer) cmd

	// cached variables
	wsAppName                    string
	pipeline                     *workspace.PipelineManifest
	shouldPromptUpdateConnection bool
	pipelineMft                  *manifest.Pipeline
	svcBuffer                    *bytes.Buffer
	jobBuffer                    *bytes.Buffer
}

func newDeployPipelineOpts(vars deployPipelineVars) (*deployPipelineOpts, error) {
	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline deploy")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))

	app, err := store.GetApplication(vars.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", vars.appName, err)
	}
	prompter := prompt.New()

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	return &deployPipelineOpts{
		app:                app,
		ws:                 ws,
		pipelineDeployer:   deploycfn.New(defaultSession),
		region:             aws.StringValue(defaultSession.Config.Region),
		deployPipelineVars: vars,
		store:              store,
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:             prompter,
		sel:                selector.NewWsPipelineSelect(prompter, store, ws),
		codestar:           cs.New(defaultSession),
		newSvcListCmd: func(w io.Writer) cmd {
			return &listSvcOpts{
				listWkldVars: listWkldVars{
					appName: vars.appName,
				},
				sel: selector.NewSelect(prompt.New(), store),
				list: &list.SvcListWriter{
					Ws:    ws,
					Store: store,
					Out:   w,

					ShowLocalSvcs: true,
					OutputJSON:    true,
				},
			}
		},
		newJobListCmd: func(w io.Writer) cmd {
			return &listJobOpts{
				listWkldVars: listWkldVars{
					appName: vars.appName,
				},
				sel: selector.NewSelect(prompt.New(), store),
				list: &list.JobListWriter{
					Ws:    ws,
					Store: store,
					Out:   w,

					ShowLocalJobs: true,
					OutputJSON:    true,
				},
			}
		},
		wsAppName: tryReadingAppName(),
		svcBuffer: &bytes.Buffer{},
		jobBuffer: &bytes.Buffer{},
	}, nil
}

// Validate returns an error if the optional flag values passed by the user are invalid.
func (o *deployPipelineOpts) Validate() error {
	if o.wsAppName == "" {
		return errNoAppInWorkspace
	}
	// The passed-in app name value must be the same as the workspace app name, as we need to be in the correct workspace to read the pipeline manifest.
	if o.appName != "" {
		if o.appName != o.wsAppName {
			return fmt.Errorf("cannot specify app %s because the workspace is already registered with app %s", o.appName, o.wsAppName)
		}
		// Validate the app name.
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return fmt.Errorf("get application %s configuration: %w", o.appName, err)
		}
	}
	o.appName = o.wsAppName
	return nil
}

// Ask prompts the user for any unprovided required fields and validates them.
func (o *deployPipelineOpts) Ask() error {
	if o.name != "" {
		if err := o.validatePipelineName(); err != nil {
			return err
		}
	} else {
		if err := o.askPipelineName(); err != nil {
			return err
		}
	}

	return nil
}

// Execute creates a new pipeline or updates the current pipeline if it already exists.
func (o *deployPipelineOpts) Execute() error {
	// bootstrap pipeline resources
	o.prog.Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, color.HighlightUserInput(o.appName)))
	err := o.pipelineDeployer.AddPipelineResourcesToApp(o.app, o.region)
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtPipelineDeployResourcesFailed, color.HighlightUserInput(o.appName)))
		return fmt.Errorf("add pipeline resources to application %s in %s: %w", o.appName, o.region, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, color.HighlightUserInput(o.appName)))

	// Read pipeline manifest.
	pipeline, err := o.getPipelineMft()
	if err != nil {
		return err
	}

	// If the source has an existing connection, get the correlating ConnectionARN .
	connection, ok := pipeline.Source.Properties["connection_name"]
	if ok {
		arn, err := o.codestar.GetConnectionARN((connection).(string))
		if err != nil {
			return fmt.Errorf("get connection ARN: %w", err)
		}
		pipeline.Source.Properties["connection_arn"] = arn
	}

	source, bool, err := deploy.PipelineSourceFromManifest(pipeline.Source)
	if err != nil {
		return fmt.Errorf("read source from manifest: %w", err)
	}
	o.shouldPromptUpdateConnection = bool

	// convert environments to deployment stages
	stages, err := o.convertStages(pipeline.Stages)
	if err != nil {
		return fmt.Errorf("convert environments to deployment stage: %w", err)
	}

	// get cross-regional resources
	artifactBuckets, err := o.getArtifactBuckets()
	if err != nil {
		return fmt.Errorf("get cross-regional resources: %w", err)
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		AppName:         o.appName,
		Name:            pipeline.Name,
		Source:          source,
		Build:           deploy.PipelineBuildFromManifest(pipeline.Build),
		Stages:          stages,
		ArtifactBuckets: artifactBuckets,
		AdditionalTags:  o.app.Tags,
	}

	if err := o.deployPipeline(deployPipelineInput); err != nil {
		return err
	}

	return nil
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

func (o *deployPipelineOpts) askPipelineName() error {
	if o.name != "" {
		return nil
	}
	pipeline, err := o.sel.Pipeline(pipelineSelectPrompt, "")
	if err != nil {
		return fmt.Errorf("select pipeline: %w", err)
	}
	o.pipeline = pipeline

	return nil
}

func (o *deployPipelineOpts) getPipelineMft() (*manifest.Pipeline, error) {
	if o.pipelineMft != nil {
		return o.pipelineMft, nil
	}
	path, err := o.ws.PipelineManifestLegacyPath()
	if err != nil {
		return nil, fmt.Errorf("get pipeline manifest path: %w", err)
	}
	pipelineMft, err := o.ws.ReadPipelineManifest(path)
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

		pipelineStage := deploy.PipelineStage{
			LocalWorkloads: workloads,
			AssociatedEnvironment: &deploy.AssociatedEnvironment{
				Name:      stage.Name,
				Region:    env.Region,
				AccountID: env.AccountID,
			},
			RequiresApproval: stage.RequiresApproval,
			TestCommands:     stage.TestCommands,
		}
		stages = append(stages, pipelineStage)
	}

	return stages, nil
}

func (o deployPipelineOpts) getLocalWorkloads() ([]string, error) {
	var localWklds []string
	if err := o.newSvcListCmd(o.svcBuffer).Execute(); err != nil {
		return nil, fmt.Errorf("get local services: %w", err)
	}
	if err := o.newJobListCmd(o.jobBuffer).Execute(); err != nil {
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

func (o *deployPipelineOpts) deployPipeline(in *deploy.CreatePipelineInput) error {
	exist, err := o.pipelineDeployer.PipelineExists(in)
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
		if err := o.pipelineDeployer.CreatePipeline(in, bucketName); err != nil {
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
	shouldUpdate, err := o.shouldUpdate()
	if err != nil {
		return err
	}
	if !shouldUpdate {
		return nil
	}
	o.prog.Start(fmt.Sprintf(fmtPipelineDeployProposalStart, color.HighlightUserInput(o.pipeline.Name)))
	if err := o.pipelineDeployer.UpdatePipeline(in, bucketName); err != nil {
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
	return cmd
}
