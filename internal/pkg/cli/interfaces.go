// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"io"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/aws/aws-sdk-go/aws/session"
)

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	// Validate returns an error if a flag's value is invalid.
	Validate() error

	// Ask prompts for flag values that are required but not passed in.
	Ask() error

	// Execute runs the command after collecting all required options.
	Execute() error

	// RecommendedActions returns a list of follow-up suggestions users can run once the command executes successfully.
	RecommendedActions() []string
}

type projectService interface {
	archer.ProjectStore
	archer.EnvironmentStore
	archer.ApplicationStore
}

type ecrService interface {
	GetRepository(name string) (string, error)
	GetECRAuth() (ecr.Auth, error)
}

type cwlogService interface {
	TaskLogEvents(logGroupName string, streamLastEventTime map[string]int64, opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
	LogGroupExists(logGroupName string) (bool, error)
}

type templater interface {
	Template() (string, error)
}

type stackSerializer interface {
	templater
	SerializedParameters() (string, error)
}

type dockerService interface {
	Build(uri, tag, path string) error
	Login(uri, username, password string) error
	Push(uri, tag string) error
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

type defaultSessionProvider interface {
	Default() (*session.Session, error)
}

type regionalSessionProvider interface {
	DefaultWithRegion(region string) (*session.Session, error)
}

type sessionFromRoleProvider interface {
	FromRole(roleARN string, region string) (*session.Session, error)
}

type profileNames interface {
	Names() []string
}

type sessionProvider interface {
	defaultSessionProvider
	regionalSessionProvider
	sessionFromRoleProvider
}

type webAppDescriber interface {
	URI(envName string) (*describe.WebAppURI, error)
	ECSParams(envName string) (*describe.WebAppECSParams, error)
	EnvVars(env *archer.Environment) ([]*describe.WebAppEnvVars, error)
	StackResources(envName string) ([]*describe.CfnResource, error)
}

type storeReader interface {
	archer.ProjectLister
	archer.ProjectGetter
	archer.EnvironmentLister
	archer.EnvironmentGetter
	archer.ApplicationLister
	archer.ApplicationGetter
}

type wsAppManifestReader interface {
	ReadAppManifest(appName string) ([]byte, error)
}

type wsAppManifestWriter interface {
	WriteAppManifest(marshaler encoding.BinaryMarshaler, appName string) (string, error)
}

type wsPipelineManifestReader interface {
	ReadPipelineManifest() ([]byte, error)
}

type wsPipelineWriter interface {
	WritePipelineBuildspec(marshaler encoding.BinaryMarshaler) (string, error)
	WritePipelineManifest(marshaler encoding.BinaryMarshaler) (string, error)
}

type wsAppDeleter interface {
	DeleteApp(name string) error
}

type wsAppReader interface {
	AppNames() ([]string, error)
	wsAppManifestReader
}

type wsPipelineDeleter interface {
	DeletePipelineManifest() error
	wsPipelineManifestReader
}

type wsPipelineReader interface {
	AppNames() ([]string, error)
	wsPipelineManifestReader
}

type wsProjectManager interface {
	Create(projectName string) error
	Summary() (*workspace.Summary, error)
}

type artifactPutter interface {
	PutArtifact(bucket, fileName string, data io.Reader) (string, error)
}

type port interface {
	Set(number int) error
}

type bucketEmptier interface {
	EmptyBucket(bucket string) error
}

// Interfaces for deploying resources through CloudFormation. Facilitates mocking.
type environmentDeployer interface {
	DeployEnvironment(env *deploy.CreateEnvironmentInput) error
	StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse)
	DeleteEnvironment(projName, envName string) error
	GetEnvironment(projectName, envName string) (*archer.Environment, error)
}

type appDeployer interface {
	// DeployApp // TODO ADD
	DeleteApp(in deploy.DeleteAppInput) error
}

type appRemover interface {
	RemoveAppFromProject(project *archer.Project, appName string) error
}

type imageRemover interface {
	ClearRepository(repoName string) error // implemented by ECR Service
}

type pipelineDeployer interface {
	CreatePipeline(env *deploy.CreatePipelineInput) error
	UpdatePipeline(env *deploy.CreatePipelineInput) error
	PipelineExists(env *deploy.CreatePipelineInput) (bool, error)
	DeletePipeline(pipelineName string) error
	AddPipelineResourcesToProject(project *archer.Project, region string) error
	projectResourcesGetter
	// TODO: Add StreamPipelineCreation method
}

type projectDeployer interface {
	DeployProject(in *deploy.CreateProjectInput) error
	AddAppToProject(project *archer.Project, appName string) error
	AddEnvToProject(project *archer.Project, env *archer.Environment) error
	DelegateDNSPermissions(project *archer.Project, accountID string) error
	DeleteProject(name string) error
}

type projectResourcesGetter interface {
	GetProjectResourcesByRegion(project *archer.Project, region string) (*archer.ProjectRegionalResources, error)
	GetRegionalProjectResources(project *archer.Project) ([]*archer.ProjectRegionalResources, error)
}

type deployer interface {
	environmentDeployer
	projectDeployer
	pipelineDeployer
}

type domainValidator interface {
	DomainExists(domainName string) (bool, error)
}

type serviceArnGetter interface {
	GetServiceArn(envName string) (*ecs.ServiceArn, error)
}

type statusDescriber interface {
	Describe() (*describe.WebAppStatusDesc, error)
}
