// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"

	"github.com/aws/aws-sdk-go/aws/session"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/task"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
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
	IsServiceDeployed(appName, envName string, svcName string) (bool, error)
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
	DeleteSecret(secretName string) error
}

type imageBuilderPusher interface {
	BuildAndPush(docker repository.ContainerLoginBuildPusher, args *exec.BuildArguments) (string, error)
}

type repositoryURIGetter interface {
	URI() string
}

type repositoryService interface {
	repositoryURIGetter
	imageBuilderPusher
}

type logEventsWriter interface {
	WriteLogEvents(opts logging.WriteLogEventsOpts) error
}

type templater interface {
	Template() (string, error)
}

type stackSerializer interface {
	templater
	SerializedParameters() (string, error)
}

type runner interface {
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

type wsFileDeleter interface {
	DeleteWorkspaceFile() error
}

type svcManifestReader interface {
	ReadServiceManifest(svcName string) ([]byte, error)
}

type jobManifestReader interface {
	ReadJobManifest(jobName string) ([]byte, error)
}

type copilotDirGetter interface {
	CopilotDirPath() (string, error)
}

type wsPipelineManifestReader interface {
	ReadPipelineManifest() ([]byte, error)
}

type wsPipelineWriter interface {
	WritePipelineBuildspec(marshaler encoding.BinaryMarshaler) (string, error)
	WritePipelineManifest(marshaler encoding.BinaryMarshaler) (string, error)
}

type wsServiceLister interface {
	ServiceNames() ([]string, error)
}

type wsSvcReader interface {
	wsServiceLister
	svcManifestReader
}

type wsSvcDirReader interface {
	wsSvcReader
	copilotDirGetter
}

type wsJobLister interface {
	JobNames() ([]string, error)
}

type wsJobReader interface {
	jobManifestReader
	wsJobLister
}

type wsWlReader interface {
	WorkloadNames() ([]string, error)
}

type wsJobDirReader interface {
	wsJobReader
	copilotDirGetter
}

type wsWlDirReader interface {
	wsJobReader
	wsSvcReader
	copilotDirGetter
	wsWlReader
	ListDockerfiles() ([]string, error)
	Summary() (*workspace.Summary, error)
}

type wsPipelineReader interface {
	wsPipelineManifestReader
	WorkloadNames() ([]string, error)
}

type wsAppManager interface {
	Create(appName string) error
	Summary() (*workspace.Summary, error)
}

type wsAddonManager interface {
	WriteAddon(f encoding.BinaryMarshaler, svc, name string) (string, error)
	wsWlReader
}

type artifactUploader interface {
	PutArtifact(bucket, fileName string, data io.Reader) (string, error)
}

type zipAndUploader interface {
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

type customResourcesUploader interface {
	UploadEnvironmentCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
}

type bucketEmptier interface {
	EmptyBucket(bucket string) error
}

// Interfaces for deploying resources through CloudFormation. Facilitates mocking.
type environmentDeployer interface {
	DeployAndRenderEnvironment(out termprogress.FileWriter, env *deploy.CreateEnvironmentInput) error
	DeleteEnvironment(appName, envName, cfnExecRoleARN string) error
	GetEnvironment(appName, envName string) (*config.Environment, error)
	EnvironmentTemplate(appName, envName string) (string, error)
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
	CreatePipeline(env *deploy.CreatePipelineInput, bucketName string) error
	UpdatePipeline(env *deploy.CreatePipelineInput, bucketName string) error
	PipelineExists(env *deploy.CreatePipelineInput) (bool, error)
	DeletePipeline(pipelineName string) error
	AddPipelineResourcesToApp(app *config.Application, region string) error
	appResourcesGetter
	// TODO: Add StreamPipelineCreation method
}

type appDeployer interface {
	DeployApp(in *deploy.CreateAppInput) error
	AddServiceToApp(app *config.Application, svcName string) error
	AddJobToApp(app *config.Application, jobName string) error
	AddEnvToApp(opts *cloudformation.AddEnvToAppOpts) error
	DelegateDNSPermissions(app *config.Application, accountID string) error
	DeleteApp(name string) error
}

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
	GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error)
}

type taskDeployer interface {
	DeployTask(out termprogress.FileWriter, input *deploy.CreateTaskResourcesInput, opts ...awscloudformation.StackOption) error
}

type taskStackManager interface {
	DeleteTask(task deploy.TaskStackInfo) error
	GetTaskStack(taskName string) (*deploy.TaskStackInfo, error)
}

type taskRunner interface {
	Run() ([]*task.Task, error)
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
	DomainHostedZoneID(domainName string) (string, error)
}

type dockerfileParser interface {
	GetExposedPorts() ([]uint16, error)
	GetHealthCheck() (*exec.HealthCheck, error)
}

type statusDescriber interface {
	Describe() (describe.HumanJSONStringer, error)
}

type envDescriber interface {
	Describe() (*describe.EnvDescription, error)
}

type versionGetter interface {
	Version() (string, error)
}

type endpointGetter interface {
	ServiceDiscoveryEndpoint() (string, error)
}

type envTemplater interface {
	EnvironmentTemplate(appName, envName string) (string, error)
}

type envUpgrader interface {
	UpgradeEnvironment(in *deploy.CreateEnvironmentInput) error
}

type legacyEnvUpgrader interface {
	UpgradeLegacyEnvironment(in *deploy.CreateEnvironmentInput, lbWebServices ...string) error
	envTemplater
}

type envTemplateUpgrader interface {
	envUpgrader
	legacyEnvUpgrader
}

type appUpgrader interface {
	UpgradeApplication(in *deploy.CreateAppInput) error
}

type pipelineGetter interface {
	GetPipeline(pipelineName string) (*codepipeline.Pipeline, error)
	ListPipelineNamesByTags(tags map[string]string) ([]string, error)
	GetPipelinesByTags(tags map[string]string) ([]*codepipeline.Pipeline, error)
}

type executor interface {
	Execute() error
}

type deletePipelineRunner interface {
	Run() error
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
	Environment(prompt, help, app string, additionalOpts ...string) (string, error)
}

type configSelector interface {
	appEnvSelector
	Service(prompt, help, app string) (string, error)
	Job(prompt, help, app string) (string, error)
}

type deploySelector interface {
	appSelector
	DeployedService(prompt, help string, app string, opts ...selector.GetDeployedServiceOpts) (*selector.DeployedService, error)
}

type pipelineSelector interface {
	Environments(prompt, help, app string, finalMsgFunc func(int) prompt.PromptConfig) ([]string, error)
}

type wsSelector interface {
	appEnvSelector
	Service(prompt, help string) (string, error)
	Job(prompt, help string) (string, error)
	Workload(msg, help string) (string, error)
}

type initJobSelector interface {
	dockerfileSelector
	Schedule(scheduleTypePrompt, scheduleTypeHelp string, scheduleValidator, rateValidator prompt.ValidatorFunc) (string, error)
}

type cfTaskSelector interface {
	Task(prompt, help string, opts ...selector.GetDeployedTaskOpts) (string, error)
}

type dockerfileSelector interface {
	Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pv prompt.ValidatorFunc) (string, error)
}

type ec2Selector interface {
	VPC(prompt, help string) (string, error)
	PublicSubnets(prompt, help, vpcID string) ([]string, error)
	PrivateSubnets(prompt, help, vpcID string) ([]string, error)
}

type credsSelector interface {
	Creds(prompt, help string) (*session.Session, error)
}

type ec2Client interface {
	HasDNSSupport(vpcID string) (bool, error)
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

type roleDeleter interface {
	DeleteRole(string) error
}

type serviceDescriber interface {
	DescribeService(app, env, svc string) (*ecs.ServiceDesc, error)
}

type apprunnerServiceDescriber interface {
	ServiceARN() (string, error)
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
	StopWorkloadTasks(app, env, workload string, stopMessage ...string) error
	StopTasksWithTaskIds(app string, env string, taskIDs[] string, stopMessage ...string) error
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
	CLIString() string
}

type secretPutter interface {
	PutSecret(in ssm.PutSecretInput) (*ssm.PutSecretOutput, error)
}

type servicePauser interface {
	PauseService(svcARN string) error
}
