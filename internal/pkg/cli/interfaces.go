// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding"
	"io"

	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/aws-sdk-go/aws/session"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	stackdescr "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

type cmd interface {
	// Validate returns an error if a flag's value is invalid.
	Validate() error

	// Ask prompts for flag values that are required but not passed in.
	Ask() error

	// Execute runs the command after collecting all required options.
	Execute() error
}

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	cmd
	// RecommendActions logs a list of follow-up suggestions users can run once the command executes successfully.
	RecommendActions() error
}

// SSM store interfaces.

type serviceStore interface {
	CreateService(svc *config.Workload) error
	GetService(appName, svcName string) (*config.Workload, error)
	ListServices(appName string) ([]*config.Workload, error)
	DeleteService(appName, svcName string) error
}

type jobStore interface {
	CreateJob(job *config.Workload) error
	GetJob(appName, jobName string) (*config.Workload, error)
	ListJobs(appName string) ([]*config.Workload, error)
	DeleteJob(appName, jobName string) error
}

type wlStore interface {
	ListWorkloads(appName string) ([]*config.Workload, error)
	GetWorkload(appName, name string) (*config.Workload, error)
}

type workloadListWriter interface {
	Write(appName string) error
}

type applicationStore interface {
	applicationCreator
	applicationUpdater
	applicationGetter
	applicationLister
	applicationDeleter
}

type applicationCreator interface {
	CreateApplication(app *config.Application) error
}

type applicationUpdater interface {
	UpdateApplication(app *config.Application) error
}

type applicationGetter interface {
	GetApplication(appName string) (*config.Application, error)
}

type applicationLister interface {
	ListApplications() ([]*config.Application, error)
}

type applicationDeleter interface {
	DeleteApplication(name string) error
}

type environmentStore interface {
	environmentCreator
	environmentGetter
	environmentLister
	environmentDeleter
	applicationGetter
}

type environmentCreator interface {
	CreateEnvironment(env *config.Environment) error
}

type environmentGetter interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
}

type environmentLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
}

type wsEnvironmentsLister interface {
	ListEnvironments() ([]string, error)
}

type environmentDeleter interface {
	DeleteEnvironment(appName, environmentName string) error
}

type store interface {
	applicationStore
	environmentStore
	serviceStore
	jobStore
	wlStore
}

type deployedEnvironmentLister interface {
	ListEnvironmentsDeployedTo(appName, svcName string) ([]string, error)
	ListDeployedServices(appName, envName string) ([]string, error)
	ListDeployedJobs(appName string, envName string) ([]string, error)
	IsServiceDeployed(appName, envName string, svcName string) (bool, error)
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

// Secretsmanager interface.

type secretsManager interface {
	secretCreator
	secretDeleter
}

type secretCreator interface {
	CreateSecret(secretName, secretString string) (string, error)
}

type secretDeleter interface {
	DescribeSecret(secretName string) (*secretsmanager.DescribeSecretOutput, error)
	DeleteSecret(secretName string) error
}

type imageBuilderPusher interface {
	BuildAndPush(ctx context.Context, args *dockerengine.BuildArguments, w io.Writer) (string, error)
	Build(ctx context.Context, args *dockerengine.BuildArguments, w io.Writer) (string, error)
}

type repositoryLogin interface {
	Login() (string, error)
}

type repositoryService interface {
	repositoryLogin
	imageBuilderPusher
}

type ecsClient interface {
	TaskDefinition(app, env, svc string) (*awsecs.TaskDefinition, error)
	ServiceConnectServices(app, env, svc string) ([]*awsecs.Service, error)
	DescribeService(app, env, svc string) (*ecs.ServiceDesc, error)
}

type logEventsWriter interface {
	WriteLogEvents(opts logging.WriteLogEventsOpts) error
}

type execRunner interface {
	Run(name string, args []string, options ...exec.CmdOption) error
}

type eventsWriter interface {
	WriteEventsUntilStopped() error
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

type sessionFromStaticProvider interface {
	FromStaticCreds(accessKeyID, secretAccessKey, sessionToken string) (*session.Session, error)
}

type sessionFromProfileProvider interface {
	FromProfile(name string) (*session.Session, error)
}

type sessionProvider interface {
	defaultSessionProvider
	regionalSessionProvider
	sessionFromRoleProvider
	sessionFromProfileProvider
	sessionFromStaticProvider
}

type describer interface {
	Describe() (describe.HumanJSONStringer, error)
}

type workloadDescriber interface {
	describer
	Manifest(string) ([]byte, error)
}

type wsFileDeleter interface {
	DeleteWorkspaceFile() error
}

type manifestReader interface {
	ReadWorkloadManifest(name string) (workspace.WorkloadManifest, error)
}

type environmentManifestWriter interface {
	WriteEnvironmentManifest(encoding.BinaryMarshaler, string) (string, error)
}

type workspacePathGetter interface {
	Path() string
}

type wsPipelineManifestReader interface {
	ReadPipelineManifest(path string) (*manifest.Pipeline, error)
}

type relPath interface {
	// Rel returns the path relative from the object's root path to the target path.
	//
	// Unlike filepath.Rel, the input path is allowed to be either relative to the
	// current working directory or absolute.
	Rel(path string) (string, error)
}

type wsPipelineIniter interface {
	relPath
	WritePipelineBuildspec(marshaler encoding.BinaryMarshaler, name string) (string, error)
	WritePipelineManifest(marshaler encoding.BinaryMarshaler, name string) (string, error)
	ListPipelines() ([]workspace.PipelineManifest, error)
}

type serviceLister interface {
	ListServices() ([]string, error)
}

type wsSvcReader interface {
	serviceLister
	manifestReader
}

type jobLister interface {
	ListJobs() ([]string, error)
}

type wsJobReader interface {
	manifestReader
	jobLister
}

type wlLister interface {
	ListWorkloads() ([]string, error)
}

type wsWorkloadReader interface {
	manifestReader
	ReadFile(path string) ([]byte, error)
	WorkloadExists(name string) (bool, error)
	WorkloadAddonFilePath(wkldName, fName string) string
	WorkloadAddonFileAbsPath(wkldName, fName string) string
}

type wsWorkloadReadWriter interface {
	wsWorkloadReader
	wsWriter
}

type wsReadWriter interface {
	wsWorkloadReadWriter
	wsEnvironmentReader
}

type wsJobDirReader interface {
	wsJobReader
	workspacePathGetter
}

type wsWlDirReader interface {
	wsJobReader
	wsSvcReader
	workspacePathGetter
	wlLister
	wsEnvironmentsLister
	WorkloadOverridesPath(string) string
	Summary() (*workspace.Summary, error)
}

type wsEnvironmentReader interface {
	wsEnvironmentsLister
	HasEnvironments() (bool, error)
	EnvOverridesPath() string
	ReadEnvironmentManifest(mftDirName string) (workspace.EnvironmentManifest, error)
	EnvAddonFilePath(fName string) string
	EnvAddonFileAbsPath(fName string) string
}

type wsPipelineReader interface {
	wsPipelineGetter
	wsPipelineManifestReader
	relPath
	PipelineOverridesPath(string) string
}

type wsPipelineGetter interface {
	wsPipelineManifestReader
	wlLister
	ListPipelines() ([]workspace.PipelineManifest, error)
}

type wsAppManager interface {
	Summary() (*workspace.Summary, error)
}

type wsAppManagerDeleter interface {
	wsAppManager
	wsFileDeleter
}

type wsWriter interface {
	Write(content encoding.BinaryMarshaler, path string) (string, error)
}

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
}

type bucketEmptier interface {
	EmptyBucket(bucket string) error
}

type stackDescriber interface {
	Resources() ([]*stackdescr.Resource, error)
}

// Interfaces for deploying resources through CloudFormation. Facilitates mocking.
type environmentDeployer interface {
	CreateAndRenderEnvironment(conf cloudformation.StackConfiguration, bucketARN string) error
	DeleteEnvironment(appName, envName, cfnExecRoleARN string) error
	GetEnvironment(appName, envName string) (*config.Environment, error)
	Template(stackName string) (string, error)
	UpdateEnvironmentTemplate(appName, envName, templateBody, cfnExecRoleARN string) error
}

type wlDeleter interface {
	DeleteWorkload(in deploy.DeleteWorkloadInput) error
}

type svcRemoverFromApp interface {
	RemoveServiceFromApp(app *config.Application, svcName string) error
}

type jobRemoverFromApp interface {
	RemoveJobFromApp(app *config.Application, jobName string) error
}

type imageRemover interface {
	ClearRepository(repoName string) error // implemented by ECR Service
}

type pipelineDeployer interface {
	CreatePipeline(bucketName string, stackConfig cloudformation.StackConfiguration) error
	UpdatePipeline(bucketName string, stackConfig cloudformation.StackConfiguration) error
	PipelineExists(stackConfig cloudformation.StackConfiguration) (bool, error)
	DeletePipeline(pipeline deploy.Pipeline) error
	AddPipelineResourcesToApp(app *config.Application, region string) error
	Template(stackName string) (string, error)
	appResourcesGetter
	// TODO: Add StreamPipelineCreation method
}

type appDeployer interface {
	DeployApp(in *deploy.CreateAppInput) error
	AddServiceToApp(app *config.Application, svcName string, opts ...cloudformation.AddWorkloadToAppOpt) error
	AddJobToApp(app *config.Application, jobName string, opts ...cloudformation.AddWorkloadToAppOpt) error
	AddEnvToApp(opts *cloudformation.AddEnvToAppOpts) error
	DelegateDNSPermissions(app *config.Application, accountID string) error
	DeleteApp(name string) error
}

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
	GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error)
}

type envDeleterFromApp interface {
	appResourcesGetter
	RemoveEnvFromApp(opts *cloudformation.RemoveEnvFromAppOpts) error
}

type taskDeployer interface {
	DeployTask(input *deploy.CreateTaskResourcesInput, opts ...awscloudformation.StackOption) error
	GetTaskStack(taskName string) (*deploy.TaskStackInfo, error)
}

type taskStackManager interface {
	DeleteTask(task deploy.TaskStackInfo) error
	GetTaskStack(taskName string) (*deploy.TaskStackInfo, error)
}

type taskRunner interface {
	Run() ([]*task.Task, error)
	CheckNonZeroExitCode([]*task.Task) error
}

type defaultClusterGetter interface {
	HasDefaultCluster() (bool, error)
}

type deployer interface {
	environmentDeployer
	appDeployer
	pipelineDeployer
	ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error)
}

type domainHostedZoneGetter interface {
	PublicDomainHostedZoneID(domainName string) (string, error)
	ValidateDomainOwnership(domainName string) error
}

type dockerfileParser interface {
	GetExposedPorts() ([]dockerfile.Port, error)
	GetHealthCheck() (*dockerfile.HealthCheck, error)
}

type statusDescriber interface {
	Describe() (describe.HumanJSONStringer, error)
}

type envDescriber interface {
	Describe() (*describe.EnvDescription, error)
	Manifest() ([]byte, error)
	ValidateCFServiceDomainAliases() error
}

type versionCompatibilityChecker interface {
	versionGetter
	AvailableFeatures() ([]string, error)
}

type versionGetter interface {
	Version() (string, error)
}

type appUpgrader interface {
	UpgradeApplication(in *deploy.CreateAppInput) error
}

type pipelineGetter interface {
	GetPipeline(pipelineName string) (*codepipeline.Pipeline, error)
}

type deployedPipelineLister interface {
	ListDeployedPipelines(appName string) ([]deploy.Pipeline, error)
}

type executor interface {
	Execute() error
}

type executeAsker interface {
	Ask() error
	executor
}

type appSelector interface {
	Application(prompt, help string, additionalOpts ...string) (string, error)
}

type appEnvSelector interface {
	appSelector
	Environment(prompt, help, app string, additionalOpts ...prompt.Option) (string, error)
}

type cfnSelector interface {
	Resources(msg, finalMsg, help, body string) ([]template.CFNResource, error)
}

type configSelector interface {
	appEnvSelector
	Service(prompt, help, app string) (string, error)
	Job(prompt, help, app string) (string, error)
	Workload(prompt, help, app string) (string, error)
}

type deploySelector interface {
	appSelector
	DeployedService(prompt, help string, app string, opts ...selector.GetDeployedWorkloadOpts) (*selector.DeployedService, error)
	DeployedJob(prompt, help string, app string, opts ...selector.GetDeployedWorkloadOpts) (*selector.DeployedJob, error)
	DeployedWorkload(prompt, help string, app string, opts ...selector.GetDeployedWorkloadOpts) (*selector.DeployedWorkload, error)
}

type pipelineEnvSelector interface {
	Environments(prompt, help, app string, finalMsgFunc func(int) prompt.PromptConfig) ([]string, error)
}

type wsPipelineSelector interface {
	WsPipeline(prompt, help string) (*workspace.PipelineManifest, error)
}

type wsEnvironmentSelector interface {
	LocalEnvironment(msg, help string) (wl string, err error)
}

type codePipelineSelector interface {
	appSelector
	DeployedPipeline(prompt, help, app string) (deploy.Pipeline, error)
}

type wsSelector interface {
	appEnvSelector
	Service(prompt, help string) (string, error)
	Job(prompt, help string) (string, error)
	Workload(msg, help string) (string, error)
	Workloads(msg, help string) ([]string, error)
}

type staticSourceSelector interface {
	StaticSources(selPrompt, selHelp, anotherPathPrompt, anotherPathHelp string, pathValidator prompt.ValidatorFunc) ([]string, error)
}

type scheduleSelector interface {
	Schedule(scheduleTypePrompt, scheduleTypeHelp string, scheduleValidator, rateValidator prompt.ValidatorFunc) (string, error)
}

type cfTaskSelector interface {
	Task(prompt, help string, opts ...selector.GetDeployedTaskOpts) (string, error)
}

type dockerfileSelector interface {
	Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pv prompt.ValidatorFunc) (string, error)
}

type topicSelector interface {
	Topics(prompt, help, app string) ([]deploy.Topic, error)
}

type ec2Selector interface {
	VPC(prompt, help string) (string, error)
	Subnets(input selector.SubnetsInput) ([]string, error)
}

type credsSelector interface {
	Creds(prompt, help string) (*session.Session, error)
}

type ec2Client interface {
	HasDNSSupport(vpcID string) (bool, error)
	ListAZs() ([]ec2.AZ, error)
}

type serviceResumer interface {
	ResumeService(string) error
}

type jobInitializer interface {
	Job(props *initialize.JobProps) (string, error)
}

type svcInitializer interface {
	Service(props *initialize.ServiceProps) (string, error)
}

type wkldInitializerWithoutManifest interface {
	AddWorkloadToApp(appName, name, workloadType string) error
}

type roleDeleter interface {
	DeleteRole(string) error
}

type policyLister interface {
	ListPolicyNames() ([]string, error)
}

type serviceDescriber interface {
	DescribeService(app, env, svc string) (*ecs.ServiceDesc, error)
}

type apprunnerServiceDescriber interface {
	ServiceARN(env string) (string, error)
}

type ecsCommandExecutor interface {
	ExecuteCommand(in awsecs.ExecuteCommandInput) error
}

type ssmPluginManager interface {
	ValidateBinary() error
	InstallLatestBinary() error
}

type taskStopper interface {
	StopOneOffTasks(app, env, family string) error
	StopDefaultClusterTasks(familyName string) error
	StopWorkloadTasks(app, env, workload string) error
}

type serviceLinkedRoleCreator interface {
	CreateECSServiceLinkedRole() error
}

type roleTagsLister interface {
	ListRoleTags(string) (map[string]string, error)
}

type roleManager interface {
	roleTagsLister
	roleDeleter
	serviceLinkedRoleCreator
}

type stackExistChecker interface {
	Exists(string) (bool, error)
}

type runningTaskSelector interface {
	RunningTask(prompt, help string, opts ...selector.TaskOpts) (*awsecs.Task, error)
}

type dockerEngine interface {
	CheckDockerEngineRunning() error
	GetPlatform() (string, string, error)
}

type codestar interface {
	GetConnectionARN(string) (string, error)
}

type publicIPGetter interface {
	PublicIP(ENI string) (string, error)
}

type cliStringer interface {
	CLIString() (string, error)
}

type secretPutter interface {
	PutSecret(in ssm.PutSecretInput) (*ssm.PutSecretOutput, error)
}

type servicePauser interface {
	PauseService(svcARN string) error
}

type interpolator interface {
	Interpolate(s string) (string, error)
}

type workloadDeployer interface {
	UploadArtifacts() (*clideploy.UploadArtifactsOutput, error)
	GenerateCloudFormationTemplate(in *clideploy.GenerateCloudFormationTemplateInput) (
		*clideploy.GenerateCloudFormationTemplateOutput, error)
	DeployWorkload(in *clideploy.DeployWorkloadInput) (clideploy.ActionRecommender, error)
	IsServiceAvailableInRegion(region string) (bool, error)
	templateDiffer
}

type templateDiffer interface {
	DeployDiff(inTmpl string) (string, error)
}

type dockerEngineRunner interface {
	CheckDockerEngineRunning() error
	Run(context.Context, *dockerengine.RunOptions) error
	IsContainerRunning(context.Context, string) (bool, error)
	Stop(context.Context, string) error
	Build(context.Context, *dockerengine.BuildArguments, io.Writer) error
	Exec(ctx context.Context, container string, out io.Writer, cmd string, args ...string) error
	ContainerExitCode(ctx context.Context, containerName string) (int, error)
	IsContainerHealthy(ctx context.Context, containerName string) (bool, error)
	Rm(context.Context, string) error
}

type workloadStackGenerator interface {
	UploadArtifacts() (*clideploy.UploadArtifactsOutput, error)
	GenerateCloudFormationTemplate(in *clideploy.GenerateCloudFormationTemplateInput) (
		*clideploy.GenerateCloudFormationTemplateOutput, error)
	AddonsTemplate() (string, error)
	templateDiffer
}

type runner interface {
	Run() error
}

type envDeployer interface {
	DeployEnvironment(in *clideploy.DeployEnvironmentInput) error
	Validate(*manifest.Environment) error
	UploadArtifacts() (*clideploy.UploadEnvArtifactsOutput, error)
	GenerateCloudFormationTemplate(in *clideploy.DeployEnvironmentInput) (
		*clideploy.GenerateCloudFormationTemplateOutput, error)
	templateDiffer
}

type envPackager interface {
	GenerateCloudFormationTemplate(in *clideploy.DeployEnvironmentInput) (*clideploy.GenerateCloudFormationTemplateOutput, error)
	Validate(*manifest.Environment) error
	UploadArtifacts() (*clideploy.UploadEnvArtifactsOutput, error)
	AddonsTemplate() (string, error)
	templateDiffer
}

type stackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() ([]*sdkcloudformation.Parameter, error)
	Tags() []*sdkcloudformation.Tag
	SerializedParameters() (string, error)
}

type secretGetter interface {
	GetSecretValue(context.Context, string) (string, error)
}
