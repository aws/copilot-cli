// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/list"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
)

type packagePipelineVars struct {
	name    string
	appName string
}

type packagePipelineOpts struct {
	packagePipelineVars

	pipelineDeployer                pipelineDeployer
	tmplWriter                      io.WriteCloser
	ws                              wsPipelineReader
	codestar                        codestar
	store                           store
	pipelineStackConfig             func(in *deploy.CreatePipelineInput) deploycfn.StackConfiguration
	configureDeployedPipelineLister func() deployedPipelineLister
	newSvcListCmd                   func(io.Writer, string) cmd
	newJobListCmd                   func(io.Writer, string) cmd
	sessProvider                    *sessions.Provider

	//catched variables
	pipelineMft *manifest.Pipeline
	app         *config.Application
	svcBuffer   *bytes.Buffer
	jobBuffer   *bytes.Buffer
}

func newPackagePipelineOpts(vars packagePipelineVars) (*packagePipelineOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline package"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))

	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	opts := &packagePipelineOpts{
		packagePipelineVars: vars,
		pipelineDeployer:    deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		tmplWriter:          os.Stdout,
		ws:                  ws,
		store:               store,
		sessProvider:        sessProvider,
		pipelineStackConfig: func(in *deploy.CreatePipelineInput) deploycfn.StackConfiguration {
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
		svcBuffer: &bytes.Buffer{},
		jobBuffer: &bytes.Buffer{},
		configureDeployedPipelineLister: func() deployedPipelineLister {
			return deploy.NewPipelineStore(rg.New(defaultSession))
		},
	}
	return opts, nil
}

func (o *packagePipelineOpts) Execute() error {
	pipelines, err := o.ws.ListPipelines()
	if err != nil {
		return fmt.Errorf("list all pipelines in the workspace: %w", err)
	}

	var pipelinePath string
	for _, pipeline := range pipelines {
		if pipeline.Name == o.name {
			pipelinePath = pipeline.Path
			break
		}
	}
	if pipelinePath == "" {
		return fmt.Errorf("pipeline %q not found", o.name)
	}

	pipelineMft, err := o.getPipelineMft(pipelinePath)
	if err != nil {
		return err
	}

	connection, ok := pipelineMft.Source.Properties["connection_name"]
	if ok {
		arn, err := o.codestar.GetConnectionARN((connection).(string))
		if err != nil {
			return fmt.Errorf("get connection ARN: %w", err)
		}
		pipelineMft.Source.Properties["connection_arn"] = arn
	}

	source, _, err := deploy.PipelineSourceFromManifest(pipelineMft.Source)
	if err != nil {
		return fmt.Errorf("read source from manifest: %w", err)
	}

	relPath, err := o.ws.Rel(pipelinePath)
	if err != nil {
		return fmt.Errorf("convert manifest path to relative path: %w", err)
	}

	stages, err := o.convertStages(pipelineMft.Stages)
	if err != nil {
		return fmt.Errorf("convert environments to deployment stage: %w", err)
	}

	appConfig, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %s configuration: %w", o.appName, err)
	}
	o.app = appConfig

	artifactBuckets, err := o.getArtifactBuckets()
	if err != nil {
		return fmt.Errorf("get cross-regional resources: %w", err)
	}

	isLegacy, err := o.isLegacy(pipelineMft.Name)
	if err != nil {
		return err
	}

	var build deploy.Build
	if err = build.Init(pipelineMft.Build, filepath.Dir(relPath)); err != nil {
		return err
	}

	overrider, err := clideploy.NewOverrider(o.ws.PipelineOverridesPath(o.name), o.appName, "", afero.NewOsFs(), o.sessProvider)
	if err != nil {
		return err
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		AppName:             o.appName,
		Name:                o.name,
		IsLegacy:            isLegacy,
		Source:              source,
		Build:               &build,
		Stages:              stages,
		ArtifactBuckets:     artifactBuckets,
		AdditionalTags:      o.app.Tags,
		Version:             version.LatestTemplateVersion(),
		PermissionsBoundary: o.app.PermissionsBoundary,
	}

	stackConfig := deploycfn.WrapWithTemplateOverrider(o.pipelineStackConfig(deployPipelineInput), overrider)
	tpl, err := stackConfig.Template()
	if err != nil {
		return err
	}
	if _, err := o.tmplWriter.Write([]byte(tpl)); err != nil {
		return err
	}
	o.tmplWriter.Close()
	return nil
}

func (o *packagePipelineOpts) getPipelineMft(pipelinePath string) (*manifest.Pipeline, error) {
	if o.pipelineMft != nil {
		return o.pipelineMft, nil
	}

	pipelineMft, err := o.ws.ReadPipelineManifest(pipelinePath)
	if err != nil {
		return nil, fmt.Errorf("read pipeline manifest: %w", err)
	}

	if err := pipelineMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate pipeline manifest: %w", err)
	}
	o.pipelineMft = pipelineMft
	return pipelineMft, nil
}

func (o *packagePipelineOpts) isLegacy(inputName string) (bool, error) {
	lister := o.configureDeployedPipelineLister()
	pipelines, err := lister.ListDeployedPipelines(o.appName)
	if err != nil {
		return false, fmt.Errorf("list deployed pipelines for app %s: %w", o.appName, err)
	}
	for _, pipeline := range pipelines {
		if pipeline.ResourceName == inputName {
			return pipeline.IsLegacy, nil
		}
	}
	return false, nil
}

func (o *packagePipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
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

func (o packagePipelineOpts) getLocalWorkloads() ([]string, error) {
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

func (o *packagePipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
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
