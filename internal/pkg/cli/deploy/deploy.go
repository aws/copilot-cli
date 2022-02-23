// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy a workload.
package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/apprunner"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"
)

const (
	fmtForceUpdateSvcStart    = "Forcing an update for service %s from environment %s"
	fmtForceUpdateSvcFailed   = "Failed to force an update for service %s from environment %s: %v.\n"
	fmtForceUpdateSvcComplete = "Forced an update for service %s from environment %s.\n"
)

var (
	aliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s.\n",
		color.HighlightCode("http.alias"),
		color.HighlightCode("copilot app init --domain example.com"))
	fmtErrTopicSubscriptionNotAllowed = "SNS topic %s does not exist in environment %s"
	resourceNameFormat                = "%s-%s-%s-%s" // Format for copilot resource names of form app-env-svc-name
)

// ActionRecommender contains methods that output action recommendation.
type ActionRecommender interface {
	RecommendedActions() []string
}

type imageBuilderPusher interface {
	BuildAndPush(docker repository.ContainerLoginBuildPusher, args *dockerengine.BuildArguments) (string, error)
}

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

type templater interface {
	Template() (string, error)
}

type stackSerializer interface {
	templater
	SerializedParameters() (string, error)
}

type endpointGetter interface {
	ServiceDiscoveryEndpoint() (string, error)
}

type versionGetter interface {
	Version() (string, error)
}

type publicCIDRBlocksGetter interface {
	PublicCIDRBlocks() ([]string, error)
}

type customResourcesUploader interface {
	UploadEnvironmentCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
	UploadRequestDrivenWebServiceCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
	UploadNetworkLoadBalancedWebServiceCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
}

type snsTopicsLister interface {
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

type serviceDeployer interface {
	DeployService(out progress.FileWriter, conf cloudformation.StackConfiguration, bucketName string, opts ...awscloudformation.StackOption) error
}

type serviceForceUpdater interface {
	ForceUpdateService(app, env, svc string) error
	LastUpdatedAt(app, env, svc string) (time.Time, error)
}

type spinner interface {
	Start(label string)
	Stop(label string)
}

type fileReader interface {
	ReadFile(string) ([]byte, error)
}

type workloadDeployer struct {
	name          string
	app           *config.Application
	env           *config.Environment
	imageTag      string
	resources     *stack.AppRegionalResources
	mft           interface{}
	workspacePath string

	// dependencies
	fs                 fileReader
	s3Client           uploader
	templater          templater
	imageBuilderPusher imageBuilderPusher
	deployer           serviceDeployer
	endpointGetter     endpointGetter
	spinner            spinner

	// cached varibles
	defaultSess              *session.Session
	defaultSessWithEnvRegion *session.Session
	envSess                  *session.Session
	store                    *config.Store
}

// WorkloadDeployerInput is the input to for workloadDeployer constructor.
type WorkloadDeployerInput struct {
	SessionProvider *sessions.Provider
	Name            string
	App             *config.Application
	Env             *config.Environment
	ImageTag        string
	Mft             interface{}
}

// NewWorkloadDeployer is the constructor for workloadDeployer.
func newWorkloadDeployer(in *WorkloadDeployerInput) (*workloadDeployer, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	workspacePath, err := ws.Path()
	if err != nil {
		return nil, fmt.Errorf("get workspace path: %w", err)
	}
	defaultSession, err := in.SessionProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("create default: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("create env session with region %s: %w", in.Env.Region, err)
	}
	envSession, err := in.SessionProvider.FromRole(in.Env.ManagerRoleARN, in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("create env session with region %s: %w", in.Env.Region, err)
	}
	defaultSessEnvRegion, err := in.SessionProvider.DefaultWithRegion(in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("create default session with region %s: %w", in.Env.Region, err)
	}
	resources, err := cloudformation.New(defaultSession).GetAppResourcesByRegion(in.App, in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get application %s resources from region %s: %w", in.App.Name, in.Env.Region, err)
	}
	addonsSvc, err := addon.New(in.Name)
	if err != nil {
		return nil, fmt.Errorf("initiate addons service: %w", err)
	}
	repoName := fmt.Sprintf("%s/%s", in.App.Name, in.Name)
	imageBuilderPusher := repository.NewWithURI(
		ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[in.Name])
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}
	endpointGetter, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         in.App.Name,
		Env:         in.Env.Name,
		ConfigStore: store,
	})
	if err != nil {
		return nil, fmt.Errorf("initiate env describer: %w", err)
	}
	return &workloadDeployer{
		name:               in.Name,
		app:                in.App,
		env:                in.Env,
		imageTag:           in.ImageTag,
		resources:          resources,
		workspacePath:      workspacePath,
		fs:                 &afero.Afero{Fs: afero.NewOsFs()},
		s3Client:           s3.New(envSession),
		templater:          addonsSvc,
		imageBuilderPusher: imageBuilderPusher,
		deployer:           cloudformation.New(envSession),
		endpointGetter:     endpointGetter,
		spinner:            termprogress.NewSpinner(log.DiagnosticWriter),

		defaultSess:              defaultSession,
		defaultSessWithEnvRegion: defaultSessEnvRegion,
		envSess:                  envSession,
		store:                    store,

		mft: in.Mft,
	}, nil
}

type svcDeployer struct {
	*workloadDeployer
	newSvcUpdater func(func(*session.Session) serviceForceUpdater) serviceForceUpdater
	now           func() time.Time
}

func newSvcDeployer(in *WorkloadDeployerInput) (*svcDeployer, error) {
	wkldDeployer, err := newWorkloadDeployer(in)
	if err != nil {
		return nil, err
	}
	return &svcDeployer{
		workloadDeployer: wkldDeployer,
		newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
			return f(wkldDeployer.envSess)
		},
		now: time.Now,
	}, nil
}

type lbSvcDeployer struct {
	*svcDeployer
	appVersionGetter       versionGetter
	publicCIDRBlocksGetter publicCIDRBlocksGetter
	lbMft                  *manifest.LoadBalancedWebService
}

// NewLBDeployer is the constructor for lbSvcDeployer.
func NewLBDeployer(in *WorkloadDeployerInput) (*lbSvcDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	deployStore, err := deploy.NewStore(svcDeployer.store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         in.App.Name,
		Env:         in.Env.Name,
		ConfigStore: svcDeployer.store,
		DeployStore: deployStore,
	})
	if err != nil {
		return nil, fmt.Errorf("create describer for environment %s in application %s: %w", in.Env.Name, in.App.Name, err)
	}
	lbMft, ok := in.Mft.(*manifest.LoadBalancedWebService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.LoadBalancedWebServiceType)
	}
	return &lbSvcDeployer{
		svcDeployer:            svcDeployer,
		appVersionGetter:       versionGetter,
		publicCIDRBlocksGetter: envDescriber,
		lbMft:                  lbMft,
	}, nil
}

type backendSvcDeployer struct {
	*svcDeployer
	backendMft *manifest.BackendService
}

// NewBackendDeployer is the constructor for backendSvcDeployer.
func NewBackendDeployer(in *WorkloadDeployerInput) (*backendSvcDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	bsMft, ok := in.Mft.(*manifest.BackendService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.BackendServiceType)
	}
	return &backendSvcDeployer{
		svcDeployer: svcDeployer,
		backendMft:  bsMft,
	}, nil
}

type jobDeployer struct {
	*workloadDeployer
	jobMft *manifest.ScheduledJob
}

// NewJobDeployer is the constructor for jobDeployer.
func NewJobDeployer(in *WorkloadDeployerInput) (*jobDeployer, error) {
	wkldDeployer, err := newWorkloadDeployer(in)
	if err != nil {
		return nil, err
	}
	jobMft, ok := in.Mft.(*manifest.ScheduledJob)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.ScheduledJobType)
	}
	return &jobDeployer{
		workloadDeployer: wkldDeployer,
		jobMft:           jobMft,
	}, nil
}

type rdwsDeployer struct {
	*svcDeployer
	customResourceUploader customResourcesUploader
	customResourceS3Client uploader
	appVersionGetter       versionGetter
	rdwsMft                *manifest.RequestDrivenWebService
}

// NewRDWSDeployer is the constructor for RDWSDeployer.
func NewRDWSDeployer(in *WorkloadDeployerInput) (*rdwsDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	rdwsMft, ok := in.Mft.(*manifest.RequestDrivenWebService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.RequestDrivenWebServiceType)
	}
	return &rdwsDeployer{
		svcDeployer:            svcDeployer,
		customResourceUploader: template.New(),
		customResourceS3Client: s3.New(svcDeployer.defaultSessWithEnvRegion),
		appVersionGetter:       versionGetter,
		rdwsMft:                rdwsMft,
	}, nil
}

type workerSvcDeployer struct {
	*svcDeployer
	topicLister snsTopicsLister
	wsMft       *manifest.WorkerService
}

// NewWorkerSvcDeployer is the constructor for workerSvcDeployer.
func NewWorkerSvcDeployer(in *WorkloadDeployerInput) (*workerSvcDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	deployStore, err := deploy.NewStore(svcDeployer.store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	wsMft, ok := in.Mft.(*manifest.WorkerService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifest.WorkerServiceType)
	}
	return &workerSvcDeployer{
		svcDeployer: svcDeployer,
		topicLister: deployStore,
		wsMft:       wsMft,
	}, nil
}

// UploadArtifactsOutput is the output of UploadArtifacts.
type UploadArtifactsOutput struct {
	ImageDigest *string
	EnvFileARN  string
	AddonsURL   string
}

// StackRuntimeConfiguration contains runtime configuration for a workload CloudFormation stack.
type StackRuntimeConfiguration struct {
	// Use *string for three states (see https://github.com/aws/copilot-cli/pull/3268#discussion_r806060230)
	// This is mainly to keep the `workload package` behavior backward-compatible, otherwise our old pipeline buildspec would break,
	// since previously we parsed the env region from a mock ECR URL that we generated from `workload package``.
	ImageDigest *string
	EnvFileARN  string
	AddonsURL   string
	RootUserARN string
	Tags        map[string]string
}

// DeployWorkloadInput is the input of DeployWorkload.
type DeployWorkloadInput struct {
	StackRuntimeConfiguration
	ForceNewUpdate bool
}

// UploadArtifacts uploads the deployment artifacts (image, addons files, env files).
func (d *workloadDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	imageDigest, err := d.uploadContainerImage(d.imageBuilderPusher)
	if err != nil {
		return nil, err
	}
	s3Artifacts, err := d.uploadArtifactsToS3(&uploadArtifactsToS3Input{
		fs:        d.fs,
		uploader:  d.s3Client,
		templater: d.templater,
	})
	if err != nil {
		return nil, err
	}

	return &UploadArtifactsOutput{
		ImageDigest: imageDigest,
		EnvFileARN:  s3Artifacts.envFileARN,
		AddonsURL:   s3Artifacts.addonsURL,
	}, nil
}

// GenerateCloudFormationTemplateInput is the input of GenerateCloudFormationTemplate.
type GenerateCloudFormationTemplateInput struct {
	StackRuntimeConfiguration
}

// GenerateCloudFormationTemplateOutput is the output of GenerateCloudFormationTemplate.
type GenerateCloudFormationTemplateOutput struct {
	Template   string
	Parameters string
}

// GenerateCloudFormationTemplate genrates a CloudFormation template and parameters for a workload.
func (d *lbSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a load balanced web service using CloudFormation.
func (d *lbSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.ForceNewUpdate, *stackConfigOutput); err != nil {
		return nil, err
	}
	return nil, nil
}

// GenerateCloudFormationTemplate genrates a CloudFormation template and parameters for a workload.
func (d *backendSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a backend service using CloudFormation.
func (d *backendSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.ForceNewUpdate, *stackConfigOutput); err != nil {
		return nil, err
	}
	return nil, nil
}

type rdwsDeployOutput struct {
	rdwsAlias string
}

// RecommendedActions returns the recommended actions after deployment.
func (d *rdwsDeployOutput) RecommendedActions() []string {
	if d.rdwsAlias == "" {
		return nil
	}
	return []string{fmt.Sprintf(`The validation process for https://%s can take more than 15 minutes.
    Please visit %s to check the validation status.`, d.rdwsAlias, color.Emphasize("https://console.aws.amazon.com/apprunner/home"))}
}

// GenerateCloudFormationTemplate genrates a CloudFormation template and parameters for a workload.
func (d *rdwsDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a request driven web service using CloudFormation.
func (d *rdwsDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.ForceNewUpdate, stackConfigOutput.svcStackConfigurationOutput); err != nil {
		return nil, err
	}
	return &rdwsDeployOutput{
		rdwsAlias: stackConfigOutput.rdSvcAlias,
	}, nil
}

type workerSvcDeployOutput struct {
	subs []manifest.TopicSubscription
}

// RecommendedActions returns the recommended actions after deployment.
func (d *workerSvcDeployOutput) RecommendedActions() []string {
	if d.subs == nil {
		return nil
	}
	retrieveEnvVarCode := "const eventsQueueURI = process.env.COPILOT_QUEUE_URI"
	actionRetrieveEnvVar := fmt.Sprintf(
		`Update worker service code to leverage the injected environment variable "COPILOT_QUEUE_URI".
    In JavaScript you can write %s.`,
		color.HighlightCode(retrieveEnvVarCode),
	)
	recs := []string{actionRetrieveEnvVar}
	topicQueueNames := d.buildWorkerQueueNames()
	if topicQueueNames == "" {
		return recs
	}
	retrieveTopicQueueEnvVarCode := fmt.Sprintf("const {%s} = JSON.parse(process.env.COPILOT_TOPIC_QUEUE_URIS)", topicQueueNames)
	actionRetrieveTopicQueues := fmt.Sprintf(
		`You can retrieve topic-specific queues by writing
    %s.`,
		color.HighlightCode(retrieveTopicQueueEnvVarCode),
	)
	recs = append(recs, actionRetrieveTopicQueues)
	return recs
}

// GenerateCloudFormationTemplate genrates a CloudFormation template and parameters for a workload.
func (d *workerSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a worker servsice using CloudFormation.
func (d *workerSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.ForceNewUpdate, stackConfigOutput.svcStackConfigurationOutput); err != nil {
		return nil, err
	}
	return &workerSvcDeployOutput{
		subs: stackConfigOutput.subscriptions,
	}, nil
}

// GenerateCloudFormationTemplate genrates a CloudFormation template and parameters for a workload.
func (d *jobDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a job using CloudFormation.
func (d *jobDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deployer.DeployService(os.Stderr, stackConfigOutput.conf, d.resources.S3Bucket,
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN)); err != nil {
		return nil, fmt.Errorf("deploy job: %w", err)
	}
	return nil, nil
}

func (d *workloadDeployer) generateCloudFormationTemplate(conf stackSerializer) (
	*GenerateCloudFormationTemplateOutput, error) {
	tpl, err := conf.Template()
	if err != nil {
		return nil, fmt.Errorf("generate stack template: %w", err)
	}
	params, err := conf.SerializedParameters()
	if err != nil {
		return nil, fmt.Errorf("generate stack template parameters: %w", err)
	}
	return &GenerateCloudFormationTemplateOutput{
		Template:   tpl,
		Parameters: params,
	}, nil
}

func (d *svcDeployer) deploy(forceNewUpdate bool, stackConfigOutput svcStackConfigurationOutput) error {
	cmdRunAt := d.now()
	if err := d.deployer.DeployService(os.Stderr, stackConfigOutput.conf, d.resources.S3Bucket,
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN)); err != nil {
		var errEmptyCS *awscloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyCS) {
			return fmt.Errorf("deploy service: %w", err)
		}
		if !forceNewUpdate {
			log.Warningln("Set --force to force an update for the service.")
			return fmt.Errorf("deploy service: %w", err)
		}
	}
	// Force update the service if --force is set and the service is not updated by the CFN.
	if forceNewUpdate {
		lastUpdatedAt, err := stackConfigOutput.svcUpdater.LastUpdatedAt(d.app.Name, d.env.Name, d.name)
		if err != nil {
			return fmt.Errorf("get the last updated deployment time for %s: %w", d.name, err)
		}
		if cmdRunAt.After(lastUpdatedAt) {
			if err := d.forceDeploy(&forceDeployInput{
				spinner:    d.spinner,
				svcUpdater: stackConfigOutput.svcUpdater,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

type forceDeployInput struct {
	spinner    spinner
	svcUpdater serviceForceUpdater
}

func (d *workloadDeployer) forceDeploy(in *forceDeployInput) error {
	in.spinner.Start(fmt.Sprintf(fmtForceUpdateSvcStart, color.HighlightUserInput(d.name), color.HighlightUserInput(d.env.Name)))
	if err := in.svcUpdater.ForceUpdateService(d.app.Name, d.env.Name, d.name); err != nil {
		errLog := fmt.Sprintf(fmtForceUpdateSvcFailed, color.HighlightUserInput(d.name),
			color.HighlightUserInput(d.env.Name), err)
		var terr timeoutError
		if errors.As(err, &terr) {
			errLog = fmt.Sprintf("%s  Run %s to check for the fail reason.\n", errLog,
				color.HighlightCode(fmt.Sprintf("copilot svc status --name %s --env %s", d.name, d.env.Name)))
		}
		in.spinner.Stop(log.Serror(errLog))
		return fmt.Errorf("force an update for service %s: %w", d.name, err)
	}
	in.spinner.Stop(log.Ssuccessf(fmtForceUpdateSvcComplete, color.HighlightUserInput(d.name), color.HighlightUserInput(d.env.Name)))
	return nil
}

func (d *workerSvcDeployOutput) buildWorkerQueueNames() string {
	var queueNames []string
	for _, subscription := range d.subs {
		if subscription.Queue.IsEmpty() {
			continue
		}
		svc := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Service))
		topic := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Name))
		queueNames = append(queueNames, fmt.Sprintf("%s%sEventsQueue", svc, strings.Title(topic)))
	}
	return strings.Join(queueNames, ", ")
}

func (d *workloadDeployer) uploadContainerImage(imgBuilderPusher imageBuilderPusher) (*string, error) {
	required, err := manifest.DockerfileBuildRequired(d.mft)
	if err != nil {
		return nil, err
	}
	if !required {
		return nil, nil
	}
	// If it is built from local Dockerfile, build and push to the ECR repo.
	buildArg, err := buildArgs(d.name, d.imageTag, d.workspacePath, d.mft)
	if err != nil {
		return nil, err
	}
	digest, err := imgBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArg)
	if err != nil {
		return nil, fmt.Errorf("build and push image: %w", err)
	}
	return aws.String(digest), nil
}

type uploadArtifactsToS3Input struct {
	fs        fileReader
	uploader  uploader
	templater templater
}

type uploadArtifactsToS3Output struct {
	envFileARN string
	addonsURL  string
}

func (d *workloadDeployer) uploadArtifactsToS3(in *uploadArtifactsToS3Input) (uploadArtifactsToS3Output, error) {
	envFileARN, err := d.pushEnvFilesToS3Bucket(&pushEnvFilesToS3BucketInput{
		fs:       in.fs,
		uploader: in.uploader,
	})
	if err != nil {
		return uploadArtifactsToS3Output{}, err
	}
	addonsURL, err := d.pushAddonsTemplateToS3Bucket(&pushAddonsTemplateToS3BucketInput{
		templater: in.templater,
		uploader:  in.uploader,
	})
	if err != nil {
		return uploadArtifactsToS3Output{}, err
	}
	return uploadArtifactsToS3Output{
		envFileARN: envFileARN,
		addonsURL:  addonsURL,
	}, nil
}

type pushEnvFilesToS3BucketInput struct {
	fs       fileReader
	uploader uploader
}

func (d *workloadDeployer) pushEnvFilesToS3Bucket(in *pushEnvFilesToS3BucketInput) (string, error) {
	path := envFile(d.mft)
	if path == "" {
		return "", nil
	}
	content, err := in.fs.ReadFile(filepath.Join(d.workspacePath, path))
	if err != nil {
		return "", fmt.Errorf("read env file %s: %w", path, err)
	}
	reader := bytes.NewReader(content)
	url, err := in.uploader.Upload(d.resources.S3Bucket, s3.MkdirSHA256(path, content), reader)
	if err != nil {
		return "", fmt.Errorf("put env file %s artifact to bucket %s: %w", path, d.resources.S3Bucket, err)
	}
	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return "", fmt.Errorf("parse s3 url: %w", err)
	}
	// The app and environment are always within the same partition.
	partition, err := partitions.Region(d.env.Region).Partition()
	if err != nil {
		return "", err
	}
	envFileARN := s3.FormatARN(partition.ID(), fmt.Sprintf("%s/%s", bucket, key))
	return envFileARN, nil
}

type pushAddonsTemplateToS3BucketInput struct {
	templater templater
	uploader  uploader
}

func (d *workloadDeployer) pushAddonsTemplateToS3Bucket(in *pushAddonsTemplateToS3BucketInput) (string, error) {
	template, err := in.templater.Template()
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if errors.As(err, &notFoundErr) {
			// addons doesn't exist for service, the url is empty.
			return "", nil
		}
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}
	reader := strings.NewReader(template)
	url, err := in.uploader.Upload(d.resources.S3Bucket, fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, d.name), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", d.resources.S3Bucket, err)
	}
	return url, nil
}

func (d *workloadDeployer) runtimeConfig(in *StackRuntimeConfiguration) (*stack.RuntimeConfig, error) {
	endpoint, err := d.endpointGetter.ServiceDiscoveryEndpoint()
	if err != nil {
		return nil, fmt.Errorf("get service discovery endpoint: %w", err)
	}
	if in.ImageDigest == nil {
		return &stack.RuntimeConfig{
			AddonsTemplateURL:        in.AddonsURL,
			EnvFileARN:               in.EnvFileARN,
			AdditionalTags:           in.Tags,
			ServiceDiscoveryEndpoint: endpoint,
			AccountID:                d.env.AccountID,
			Region:                   d.env.Region,
		}, nil
	}
	return &stack.RuntimeConfig{
		AddonsTemplateURL: in.AddonsURL,
		EnvFileARN:        in.EnvFileARN,
		AdditionalTags:    in.Tags,
		Image: &stack.ECRImage{
			RepoURL:  d.resources.RepositoryURLs[d.name],
			ImageTag: d.imageTag,
			Digest:   aws.StringValue(in.ImageDigest),
		},
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                d.env.AccountID,
		Region:                   d.env.Region,
	}, nil
}

type svcStackConfigurationOutput struct {
	conf       cloudformation.StackConfiguration
	svcUpdater serviceForceUpdater
}

func (d *lbSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*svcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	if err := validateLBWSRuntime(d.app, d.env.Name, d.lbMft, d.appVersionGetter); err != nil {
		return nil, err
	}
	var opts []stack.LoadBalancedWebServiceOption
	if !d.lbMft.NLBConfig.IsEmpty() {
		cidrBlocks, err := d.publicCIDRBlocksGetter.PublicCIDRBlocks()
		if err != nil {
			return nil, fmt.Errorf("get public CIDR blocks information from the VPC of environment %s: %w", d.env.Name, err)
		}
		opts = append(opts, stack.WithNLB(cidrBlocks))
	}
	if d.app.RequiresDNSDelegation() {
		opts = append(opts, stack.WithDNSDelegation(deploy.AppInformation{
			Name:                d.app.Name,
			DNSName:             d.app.Domain,
			AccountPrincipalARN: in.RootUserARN,
		}))
		if !d.lbMft.RoutingRule.Disabled() {
			opts = append(opts, stack.WithHTTPS())
		}
	}
	conf, err := stack.NewLoadBalancedWebService(d.lbMft, d.env.Name, d.app.Name, *rc, opts...)
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &svcStackConfigurationOutput{
		conf: conf,
		svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
			return ecs.New(s)
		}),
	}, nil
}

func (d *backendSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*svcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	conf, err := stack.NewBackendService(d.backendMft, d.env.Name, d.app.Name, *rc)
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &svcStackConfigurationOutput{
		conf: conf,
		svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
			return ecs.New(s)
		}),
	}, nil
}

type rdwsStackConfigurationOutput struct {
	svcStackConfigurationOutput
	rdSvcAlias string
}

func (d *rdwsDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*rdwsStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}

	if d.app.Domain == "" && d.rdwsMft.Alias != nil {
		log.Errorf(aliasUsedWithoutDomainFriendlyText)
		return nil, errors.New("alias specified when application is not associated with a domain")
	}
	appInfo := deploy.AppInformation{
		Name:                d.app.Name,
		DNSName:             d.app.Domain,
		AccountPrincipalARN: in.RootUserARN,
	}
	if d.rdwsMft.Alias == nil {
		conf, err := stack.NewRequestDrivenWebService(d.rdwsMft, d.env.Name, appInfo, *rc)
		if err != nil {
			return nil, fmt.Errorf("create stack configuration: %w", err)
		}
		return &rdwsStackConfigurationOutput{
			svcStackConfigurationOutput: svcStackConfigurationOutput{
				conf: conf,
				svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
					return apprunner.New(s)
				}),
			},
		}, nil
	}

	if err = validateRDSvcAliasAndAppVersion(d.name,
		aws.StringValue(d.rdwsMft.Alias), d.env.Name, d.app, d.appVersionGetter); err != nil {
		return nil, err
	}
	var urls map[string]string
	if urls, err = uploadRDWSCustomResources(&uploadRDWSCustomResourcesInput{
		customResourceUploader: d.customResourceUploader,
		s3Uploader:             d.customResourceS3Client,
		s3Bucket:               d.resources.S3Bucket,
	}); err != nil {
		return nil, err
	}
	conf, err := stack.NewRequestDrivenWebServiceWithAlias(d.rdwsMft, d.env.Name, appInfo, *rc, urls)
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &rdwsStackConfigurationOutput{
		svcStackConfigurationOutput: svcStackConfigurationOutput{
			conf: conf,
			svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
				return apprunner.New(s)
			}),
		},
		rdSvcAlias: aws.StringValue(d.rdwsMft.Alias),
	}, nil
}

type workerSvcStackConfigurationOutput struct {
	svcStackConfigurationOutput
	subscriptions []manifest.TopicSubscription
}

func (d *workerSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*workerSvcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	var topics []deploy.Topic
	topics, err = d.topicLister.ListSNSTopics(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("get SNS topics for app %s and environment %s: %w", d.app.Name, d.env.Name, err)
	}
	var topicARNs []string
	for _, topic := range topics {
		topicARNs = append(topicARNs, topic.ARN())
	}
	subs := d.wsMft.Subscriptions()
	if err = validateTopicsExist(subs, topicARNs, d.app.Name, d.env.Name); err != nil {
		return nil, err
	}
	conf, err := stack.NewWorkerService(d.wsMft, d.env.Name, d.app.Name, *rc)
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &workerSvcStackConfigurationOutput{
		svcStackConfigurationOutput: svcStackConfigurationOutput{
			conf: conf,
			svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
				return ecs.New(s)
			}),
		},
		subscriptions: subs,
	}, nil
}

type jobStackConfigurationOutput struct {
	conf cloudformation.StackConfiguration
}

func (d *jobDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*jobStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	conf, err := stack.NewScheduledJob(d.jobMft, d.env.Name, d.app.Name, *rc)
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &jobStackConfigurationOutput{
		conf: conf,
	}, nil
}

func buildArgs(name, imageTag, workspacePath string, unmarshaledManifest interface{}) (*dockerengine.BuildArguments, error) {
	type dfArgs interface {
		BuildArgs(rootDirectory string) *manifest.DockerBuildArgs
		ContainerPlatform() string
	}
	mf, ok := unmarshaledManifest.(dfArgs)
	if !ok {
		return nil, fmt.Errorf("%s does not have required methods BuildArgs() and ContainerPlatform()", name)
	}
	var tags []string
	if imageTag != "" {
		tags = append(tags, imageTag)
	}
	args := mf.BuildArgs(workspacePath)
	return &dockerengine.BuildArguments{
		Dockerfile: *args.Dockerfile,
		Context:    *args.Context,
		Args:       args.Args,
		CacheFrom:  args.CacheFrom,
		Target:     aws.StringValue(args.Target),
		Platform:   mf.ContainerPlatform(),
		Tags:       tags,
	}, nil
}

func envFile(unmarshaledManifest interface{}) string {
	type envFile interface {
		EnvFile() string
	}
	mf, ok := unmarshaledManifest.(envFile)
	if ok {
		return mf.EnvFile()
	}
	// If the manifest type doesn't support envFiles, ignore and move forward.
	return ""
}

func validateTopicsExist(subscriptions []manifest.TopicSubscription, topicARNs []string, app, env string) error {
	validTopicResources := make([]string, 0, len(topicARNs))
	for _, topic := range topicARNs {
		parsedTopic, err := arn.Parse(topic)
		if err != nil {
			continue
		}
		validTopicResources = append(validTopicResources, parsedTopic.Resource)
	}

	for _, ts := range subscriptions {
		topicName := fmt.Sprintf(resourceNameFormat, app, env, aws.StringValue(ts.Service), aws.StringValue(ts.Name))
		if !contains(topicName, validTopicResources) {
			return fmt.Errorf(fmtErrTopicSubscriptionNotAllowed, topicName, env)
		}
	}
	return nil
}

func contains(s string, items []string) bool {
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}

type uploadRDWSCustomResourcesInput struct {
	customResourceUploader customResourcesUploader
	s3Uploader             uploader
	s3Bucket               string
}

func uploadRDWSCustomResources(in *uploadRDWSCustomResourcesInput) (map[string]string, error) {
	urls, err := in.customResourceUploader.UploadRequestDrivenWebServiceCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return in.s3Uploader.ZipAndUpload(in.s3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", in.s3Bucket, err)
	}

	return urls, nil
}

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter versionGetter) error {
	if alias == "" {
		return nil
	}
	if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
		logAppVersionOutdatedError(svcName)
		return err
	}
	// Alias should be within root hosted zone.
	aliasInvalidLog := fmt.Sprintf(`%s of %s field should match the pattern <subdomain>.%s 
Where <subdomain> cannot be the application name.
`, color.HighlightUserInput(alias), color.HighlightCode("http.alias"), app.Domain)
	if err := checkUnsupportedRDSvcAlias(alias, envName, app); err != nil {
		log.Errorf(aliasInvalidLog)
		return err
	}

	// Example: subdomain.domain
	regRootHostedZone, err := regexp.Compile(fmt.Sprintf(`^([^\.]+\.)%s`, app.Domain))
	if err != nil {
		return err
	}

	if regRootHostedZone.MatchString(alias) {
		return nil
	}

	log.Errorf(aliasInvalidLog)
	return fmt.Errorf("alias is not supported in hosted zones that are not managed by Copilot")
}

func validateAppVersion(appName string, appVersionGetter versionGetter) error {
	appVersion, err := appVersionGetter.Version()
	if err != nil {
		return fmt.Errorf("get version for app %s: %w", appName, err)
	}
	diff := semver.Compare(appVersion, deploy.AliasLeastAppTemplateVersion)
	if diff < 0 {
		return fmt.Errorf(`alias is not compatible with application versions below %s`, deploy.AliasLeastAppTemplateVersion)
	}
	return nil
}

func validateLBWSRuntime(app *config.Application, envName string, mft *manifest.LoadBalancedWebService, appVersionGetter versionGetter) error {
	if app.Domain == "" && mft.HasAliases() {
		log.Errorf(aliasUsedWithoutDomainFriendlyText)
		return errors.New("alias specified when application is not associated with a domain")
	}

	if app.RequiresDNSDelegation() {
		if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
			logAppVersionOutdatedError(aws.StringValue(mft.Name))
			return err
		}
	}

	if err := validateLBSvcAlias(mft.RoutingRule.Alias, app, envName); err != nil {
		return err
	}
	return validateLBSvcAlias(mft.NLBConfig.Aliases, app, envName)
}

func validateLBSvcAlias(aliases manifest.Alias, app *config.Application, envName string) error {
	if aliases.IsEmpty() {
		return nil
	}
	aliasList, err := aliases.ToStringSlice()
	if err != nil {
		return fmt.Errorf(`convert 'http.alias' to string slice: %w`, err)
	}
	for _, alias := range aliasList {
		// Alias should be within either env, app, or root hosted zone.
		var regEnvHostedZone, regAppHostedZone, regRootHostedZone *regexp.Regexp
		var err error
		if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
			return err
		}
		if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
			return err
		}
		if regRootHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s`, app.Domain)); err != nil {
			return err
		}
		var validAlias bool
		for _, re := range []*regexp.Regexp{regEnvHostedZone, regAppHostedZone, regRootHostedZone} {
			if re.MatchString(alias) {
				validAlias = true
				break
			}
		}
		if validAlias {
			continue
		}
		log.Errorf(`%s must match one of the following patterns:
- %s.%s.%s,
- <name>.%s.%s.%s,
- %s.%s,
- <name>.%s.%s,
- %s,
- <name>.%s
`, color.HighlightCode("http.alias"), envName, app.Name, app.Domain, envName,
			app.Name, app.Domain, app.Name, app.Domain, app.Name,
			app.Domain, app.Domain, app.Domain)
		return fmt.Errorf(`alias "%s" is not supported in hosted zones managed by Copilot`, alias)
	}
	return nil
}

func checkUnsupportedRDSvcAlias(alias, envName string, app *config.Application) error {
	var regEnvHostedZone, regAppHostedZone *regexp.Regexp
	var err error
	// Example: subdomain.env.app.domain, env.app.domain
	if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
		return err
	}

	// Example: subdomain.app.domain, app.domain
	if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
		return err
	}

	if regEnvHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an environment-level alias, which is not supported yet", alias)
	}

	if regAppHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an application-level alias, which is not supported yet", alias)
	}

	if alias == app.Domain {
		return fmt.Errorf("%s is a root domain alias, which is not supported yet", alias)
	}

	return nil
}

func logAppVersionOutdatedError(name string) {
	log.Errorf(`Cannot deploy service %s because the application version is incompatible.
To upgrade the application, please run %s first (see https://aws.github.io/copilot-cli/docs/credentials/#application-credentials).
`, name, color.HighlightCode("copilot app upgrade"))
}

type timeoutError interface {
	error
	Timeout() bool
}
