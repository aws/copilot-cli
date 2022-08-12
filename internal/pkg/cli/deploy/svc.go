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

	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/copilot-cli/internal/pkg/aws/acm"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"

	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/apprunner"
	awsapprunner "github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
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
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	fmtForceUpdateSvcStart    = "Forcing an update for service %s from environment %s"
	fmtForceUpdateSvcFailed   = "Failed to force an update for service %s from environment %s: %v.\n"
	fmtForceUpdateSvcComplete = "Forced an update for service %s from environment %s.\n"
)

var (
	rdwsAliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s.\n",
		color.HighlightCode("http.alias"),
		color.HighlightCode("copilot app init --domain example.com"))
	ecsALBAliasUsedWithoutDomainFriendlyText = fmt.Sprintf(`To use %s, your application must be:
* Associated with a domain:
  %s
* Or, using imported certificates to your environment:
  %s
`,
		color.HighlightCode("http.alias"),
		color.HighlightCode("copilot app init --domain example.com"),
		color.HighlightCode("copilot env init --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"))
	ecsNLBAliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s",
		color.HighlightCode("nlb.alias"),
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

type stackBuilder interface {
	templater
	Parameters() (string, error)
	Package(addon.PackageConfig) error
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

type snsTopicsLister interface {
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

type serviceDeployer interface {
	DeployService(conf cloudformation.StackConfiguration, bucketName string, opts ...awscloudformation.StackOption) error
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

type aliasCertValidator interface {
	ValidateCertAliases(aliases []string, certs []string) error
}

type workloadDeployer struct {
	name          string
	app           *config.Application
	env           *config.Environment
	imageTag      string
	resources     *stack.AppRegionalResources
	mft           interface{}
	rawMft        []byte
	workspacePath string

	// Dependencies.
	fs                 fileReader
	s3Client           uploader
	addons             stackBuilder
	imageBuilderPusher imageBuilderPusher
	deployer           serviceDeployer
	endpointGetter     endpointGetter
	spinner            spinner
	templateFS         template.Reader

	// Cached variables.
	defaultSess              *session.Session
	defaultSessWithEnvRegion *session.Session
	envSess                  *session.Session
	store                    *config.Store
	envConfig                *manifest.Environment
}

// WorkloadDeployerInput is the input to for workloadDeployer constructor.
type WorkloadDeployerInput struct {
	SessionProvider *sessions.Provider
	Name            string
	App             *config.Application
	Env             *config.Environment
	ImageTag        string
	Mft             interface{} // Interpolated, applied, and unmarshaled manifest.
	RawMft          []byte      // Content of the manifest file without any transformations.
}

// newWorkloadDeployer is the constructor for workloadDeployer.
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
	resources, err := cloudformation.New(defaultSession, cloudformation.WithProgressTracker(os.Stderr)).GetAppResourcesByRegion(in.App, in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get application %s resources from region %s: %w", in.App.Name, in.Env.Region, err)
	}

	var addons stackBuilder
	addons, err = addon.Parse(in.Name, ws)
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("parse addons stack: %w", err)
		}
		addons = nil // so that we can check for no addons with nil comparison
	}

	repoName := fmt.Sprintf("%s/%s", in.App.Name, in.Name)
	imageBuilderPusher := repository.NewWithURI(
		ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[in.Name])
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         in.App.Name,
		Env:         in.Env.Name,
		ConfigStore: store,
	})
	if err != nil {
		return nil, fmt.Errorf("initiate env describer: %w", err)
	}

	mft, err := envDescriber.Manifest()
	if err != nil {
		return nil, fmt.Errorf("read the manifest used to deploy environment %s: %w", in.Env.Name, err)
	}
	envConfig, err := manifest.UnmarshalEnvironment(mft)
	if err != nil {
		return nil, fmt.Errorf("unmarshal the manifest used to deploy environment %s: %w", in.Env.Name, err)
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
		addons:             addons,
		imageBuilderPusher: imageBuilderPusher,
		deployer:           cloudformation.New(envSession, cloudformation.WithProgressTracker(os.Stderr)),
		endpointGetter:     envDescriber,
		spinner:            termprogress.NewSpinner(log.DiagnosticWriter),
		templateFS:         template.New(),

		defaultSess:              defaultSession,
		defaultSessWithEnvRegion: defaultSessEnvRegion,
		envSess:                  envSession,
		store:                    store,
		envConfig:                envConfig,

		mft:    in.Mft,
		rawMft: in.RawMft,
	}, nil
}

// AddonsTemplate returns this workload's addon template.
func (w *workloadDeployer) AddonsTemplate() (string, error) {
	if w.addons == nil {
		return "", nil
	}

	return w.addons.Template()
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

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (lbWebSvcDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
}

type customResourcesFunc func(fs template.Reader) ([]*customresource.CustomResource, error)

type lbWebSvcDeployer struct {
	*svcDeployer
	appVersionGetter       versionGetter
	publicCIDRBlocksGetter publicCIDRBlocksGetter
	lbMft                  *manifest.LoadBalancedWebService
	customResources        customResourcesFunc
	newAliasCertValidator  func(optionalRegion *string) aliasCertValidator
}

// NewLBWSDeployer is the constructor for lbWebSvcDeployer.
func NewLBWSDeployer(in *WorkloadDeployerInput) (*lbWebSvcDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	deployStore, err := deploy.NewStore(in.SessionProvider, svcDeployer.store)
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
	return &lbWebSvcDeployer{
		svcDeployer:            svcDeployer,
		appVersionGetter:       versionGetter,
		publicCIDRBlocksGetter: envDescriber,
		lbMft:                  lbMft,
		newAliasCertValidator: func(optionalRegion *string) aliasCertValidator {
			sess := svcDeployer.envSess.Copy(&aws.Config{
				Region: optionalRegion,
			})
			return acm.New(sess)
		},
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.LBWS(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.LoadBalancedWebServiceType, err)
			}
			return crs, nil
		},
	}, nil
}

type backendSvcDeployer struct {
	*svcDeployer
	backendMft         *manifest.BackendService
	customResources    customResourcesFunc
	aliasCertValidator aliasCertValidator
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (backendSvcDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
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
		svcDeployer:        svcDeployer,
		backendMft:         bsMft,
		aliasCertValidator: acm.New(svcDeployer.envSess),
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.Backend(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.BackendServiceType, err)
			}
			return crs, nil
		},
	}, nil
}

type jobDeployer struct {
	*workloadDeployer
	jobMft          *manifest.ScheduledJob
	customResources customResourcesFunc
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (jobDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
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
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.ScheduledJob(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.ScheduledJobType, err)
			}
			return crs, nil
		},
	}, nil
}

type rdwsDeployer struct {
	*svcDeployer
	customResourceS3Client uploader
	appVersionGetter       versionGetter
	rdwsMft                *manifest.RequestDrivenWebService
	customResources        customResourcesFunc
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (rdwsDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsapprunner.EndpointsID, region)
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
		customResourceS3Client: s3.New(svcDeployer.defaultSessWithEnvRegion),
		appVersionGetter:       versionGetter,
		rdwsMft:                rdwsMft,
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.RDWS(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.RequestDrivenWebServiceType, err)
			}
			return crs, nil
		},
	}, nil
}

type workerSvcDeployer struct {
	*svcDeployer
	topicLister     snsTopicsLister
	wsMft           *manifest.WorkerService
	customResources customResourcesFunc
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (workerSvcDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
}

// NewWorkerSvcDeployer is the constructor for workerSvcDeployer.
func NewWorkerSvcDeployer(in *WorkloadDeployerInput) (*workerSvcDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	deployStore, err := deploy.NewStore(in.SessionProvider, svcDeployer.store)
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
		customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
			crs, err := customresource.Worker(fs)
			if err != nil {
				return nil, fmt.Errorf("read custom resources for a %q: %w", manifest.WorkerServiceType, err)
			}
			return crs, nil
		},
	}, nil
}

// UploadArtifactsOutput is the output of UploadArtifacts.
type UploadArtifactsOutput struct {
	ImageDigest        *string
	EnvFileARN         string
	AddonsURL          string
	CustomResourceURLs map[string]string
}

// StackRuntimeConfiguration contains runtime configuration for a workload CloudFormation stack.
type StackRuntimeConfiguration struct {
	// Use *string for three states (see https://github.com/aws/copilot-cli/pull/3268#discussion_r806060230)
	// This is mainly to keep the `workload package` behavior backward-compatible, otherwise our old pipeline buildspec would break,
	// since previously we parsed the env region from a mock ECR URL that we generated from `workload package``.
	ImageDigest        *string
	EnvFileARN         string
	AddonsURL          string
	RootUserARN        string
	Tags               map[string]string
	CustomResourceURLs map[string]string
}

// DeployWorkloadInput is the input of DeployWorkload.
type DeployWorkloadInput struct {
	StackRuntimeConfiguration
	Options
}

// Options specifies options for the deployment.
type Options struct {
	ForceNewUpdate  bool
	DisableRollback bool
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *lbWebSvcDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.customResources)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *backendSvcDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.customResources)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *rdwsDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.customResources)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *workerSvcDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.customResources)
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *jobDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.customResources)
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

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *lbWebSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a load balanced web service using CloudFormation.
func (d *lbWebSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.Options, *stackConfigOutput); err != nil {
		return nil, err
	}
	return nil, nil
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
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
	if err := d.deploy(in.Options, *stackConfigOutput); err != nil {
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

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
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
	if err := d.deploy(in.Options, stackConfigOutput.svcStackConfigurationOutput); err != nil {
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

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *workerSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a worker service using CloudFormation.
func (d *workerSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.Options, stackConfigOutput.svcStackConfigurationOutput); err != nil {
		return nil, err
	}
	return &workerSvcDeployOutput{
		subs: stackConfigOutput.subscriptions,
	}, nil
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
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
	opts := []awscloudformation.StackOption{
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN),
	}
	if in.DisableRollback {
		opts = append(opts, awscloudformation.WithDisableRollback())
	}
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deployer.DeployService(stackConfigOutput.conf, d.resources.S3Bucket, opts...); err != nil {
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

func (d *svcDeployer) deploy(deployOptions Options, stackConfigOutput svcStackConfigurationOutput) error {
	opts := []awscloudformation.StackOption{
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN),
	}
	if deployOptions.DisableRollback {
		opts = append(opts, awscloudformation.WithDisableRollback())
	}
	cmdRunAt := d.now()
	if err := d.deployer.DeployService(stackConfigOutput.conf, d.resources.S3Bucket, opts...); err != nil {
		var errEmptyCS *awscloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyCS) {
			return fmt.Errorf("deploy service: %w", err)
		}
		if !deployOptions.ForceNewUpdate {
			log.Warningln("Set --force to force an update for the service.")
			return fmt.Errorf("deploy service: %w", err)
		}
	}
	// Force update the service if --force is set and the service is not updated by the CFN.
	if deployOptions.ForceNewUpdate {
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
		queueNames = append(queueNames, fmt.Sprintf("%s%sEventsQueue", svc, cases.Title(language.English).String(topic)))
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
	fs       fileReader
	uploader uploader
}

type uploadArtifactsToS3Output struct {
	envFileARN string
	addonsURL  string
}

func (d *workloadDeployer) uploadArtifactsToS3(in *uploadArtifactsToS3Input) (uploadArtifactsToS3Output, error) {
	envFileARN, err := d.pushEnvFileToS3Bucket(&pushEnvFilesToS3BucketInput{
		fs:       in.fs,
		uploader: in.uploader,
	})
	if err != nil {
		return uploadArtifactsToS3Output{}, err
	}
	addonsURL, err := d.pushAddonsTemplateToS3Bucket()
	if err != nil {
		return uploadArtifactsToS3Output{}, err
	}
	return uploadArtifactsToS3Output{
		envFileARN: envFileARN,
		addonsURL:  addonsURL,
	}, nil
}

func (d *workloadDeployer) uploadArtifacts(customResources customResourcesFunc) (*UploadArtifactsOutput, error) {
	imageDigest, err := d.uploadContainerImage(d.imageBuilderPusher)
	if err != nil {
		return nil, err
	}
	s3Artifacts, err := d.uploadArtifactsToS3(&uploadArtifactsToS3Input{
		fs:       d.fs,
		uploader: d.s3Client,
	})
	if err != nil {
		return nil, err
	}

	out := &UploadArtifactsOutput{
		ImageDigest: imageDigest,
		EnvFileARN:  s3Artifacts.envFileARN,
		AddonsURL:   s3Artifacts.addonsURL,
	}
	crs, err := customResources(d.templateFS)
	if err != nil {
		return nil, err
	}
	urls, err := customresource.Upload(func(key string, contents io.Reader) (string, error) {
		return d.s3Client.Upload(d.resources.S3Bucket, key, contents)
	}, crs)
	if err != nil {
		return nil, fmt.Errorf("upload custom resources for %q: %w", d.name, err)
	}
	out.CustomResourceURLs = urls
	return out, nil
}

type pushEnvFilesToS3BucketInput struct {
	fs       fileReader
	uploader uploader
}

func (d *workloadDeployer) pushEnvFileToS3Bucket(in *pushEnvFilesToS3BucketInput) (string, error) {
	path := envFile(d.mft)
	if path == "" {
		return "", nil
	}
	content, err := in.fs.ReadFile(filepath.Join(d.workspacePath, path))
	if err != nil {
		return "", fmt.Errorf("read env file %s: %w", path, err)
	}
	reader := bytes.NewReader(content)
	url, err := in.uploader.Upload(d.resources.S3Bucket, artifactpath.EnvFiles(path, content), reader)
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

func (d *workloadDeployer) pushAddonsTemplateToS3Bucket() (string, error) {
	if d.addons == nil {
		return "", nil
	}

	config := addon.PackageConfig{
		Bucket:        d.resources.S3Bucket,
		Uploader:      d.s3Client,
		WorkspacePath: d.workspacePath,
		FS:            afero.NewOsFs(),
	}
	if err := d.addons.Package(config); err != nil {
		return "", fmt.Errorf("package addons: %w", err)
	}

	tmpl, err := d.addons.Template()
	if err != nil {
		return "", fmt.Errorf("retrieve addons template: %w", err)
	}

	reader := strings.NewReader(tmpl)
	url, err := d.s3Client.Upload(d.resources.S3Bucket, artifactpath.Addons(d.name, []byte(tmpl)), reader)
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
			CustomResourcesURL:       in.CustomResourceURLs,
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
		CustomResourcesURL:       in.CustomResourceURLs,
	}, nil
}

type svcStackConfigurationOutput struct {
	conf       cloudformation.StackConfiguration
	svcUpdater serviceForceUpdater
}

func (d *lbWebSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*svcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	if err := d.validateALBRuntime(); err != nil {
		return nil, err
	}
	if err := d.validateNLBRuntime(); err != nil {
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
	conf, err := stack.NewLoadBalancedWebService(stack.LoadBalancedWebServiceConfig{
		App:           d.app,
		EnvManifest:   d.envConfig,
		Manifest:      d.lbMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		RootUserARN:   in.RootUserARN,
		Addons:        d.addons,
	}, opts...)
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
	if err := d.validateALBRuntime(); err != nil {
		return nil, err
	}
	conf, err := stack.NewBackendService(stack.BackendServiceConfig{
		App:           d.app,
		EnvManifest:   d.envConfig,
		Manifest:      d.backendMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		Addons:        d.addons,
	})
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
		log.Errorf(rdwsAliasUsedWithoutDomainFriendlyText)
		return nil, errors.New("alias specified when application is not associated with a domain")
	}

	conf, err := stack.NewRequestDrivenWebService(stack.RequestDrivenWebServiceConfig{
		App: deploy.AppInformation{
			Name:                d.app.Name,
			Domain:              d.app.Domain,
			AccountPrincipalARN: in.RootUserARN,
		},
		Env:           d.env.Name,
		Manifest:      d.rdwsMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		Addons:        d.addons,
	})
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	if d.rdwsMft.Alias == nil {
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
	conf, err := stack.NewWorkerService(stack.WorkerServiceConfig{
		App:           d.app.Name,
		Env:           d.env.Name,
		Manifest:      d.wsMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		Addons:        d.addons,
	})
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
	conf, err := stack.NewScheduledJob(stack.ScheduledJobConfig{
		App:           d.app.Name,
		Env:           d.env.Name,
		Manifest:      d.jobMft,
		RawManifest:   d.rawMft,
		RuntimeConfig: *rc,
		Addons:        d.addons,
	})
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

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter versionGetter) error {
	if alias == "" {
		return nil
	}
	if err := validateAppVersionForAlias(app.Name, appVersionGetter); err != nil {
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

func validateAppVersionForAlias(appName string, appVersionGetter versionGetter) error {
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

func (d *backendSvcDeployer) validateALBRuntime() error {
	if d.backendMft.RoutingRule.IsEmpty() {
		return nil
	}
	hasImportedCerts := len(d.envConfig.HTTPConfig.Private.Certificates) != 0
	switch {
	case d.backendMft.RoutingRule.Alias.IsEmpty() && hasImportedCerts:
		return &errSvcWithNoALBAliasDeployingToEnvWithImportedCerts{
			name:    d.name,
			envName: d.env.Name,
		}
	case d.backendMft.RoutingRule.Alias.IsEmpty():
		return nil
	case !hasImportedCerts:
		return fmt.Errorf(`cannot specify "alias" in an environment without imported certs`)
	}

	aliases, err := d.backendMft.RoutingRule.Alias.ToStringSlice()
	if err != nil {
		return fmt.Errorf("convert aliases to string slice: %w", err)
	}

	if err := d.aliasCertValidator.ValidateCertAliases(aliases, d.envConfig.HTTPConfig.Private.Certificates); err != nil {
		return fmt.Errorf("validate aliases against the imported certificate for env %s: %w", d.env.Name, err)
	}

	return nil
}

func (d *lbWebSvcDeployer) validateALBRuntime() error {
	hasImportedCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	if d.lbMft.RoutingRule.Alias.IsEmpty() {
		if hasImportedCerts {
			return &errSvcWithNoALBAliasDeployingToEnvWithImportedCerts{
				name:    d.name,
				envName: d.env.Name,
			}
		}
		return nil
	}
	importedHostedZones := d.lbMft.RoutingRule.Alias.HostedZones()
	if len(importedHostedZones) != 0 {
		if !hasImportedCerts {
			return fmt.Errorf("cannot specify alias hosted zones %v when no certificates are imported in environment %q", importedHostedZones, d.env.Name)
		}
		if d.envConfig.CDNEnabled() {
			return &errSvcWithALBAliasHostedZoneWithCDNEnabled{
				envName: d.env.Name,
			}
		}
	}
	if hasImportedCerts {
		aliases, err := d.lbMft.RoutingRule.Alias.ToStringSlice()
		if err != nil {
			return fmt.Errorf("convert aliases to string slice: %w", err)
		}

		cdnCert := d.envConfig.CDNConfig.Config.Certificate
		albCertValidator := d.newAliasCertValidator(nil)
		cfCertValidator := d.newAliasCertValidator(aws.String(cloudfront.CertRegion))
		if err := albCertValidator.ValidateCertAliases(aliases, d.envConfig.HTTPConfig.Public.Certificates); err != nil {
			return fmt.Errorf("validate aliases against the imported public ALB certificate for env %s: %w", d.env.Name, err)
		}
		if cdnCert != nil {
			if err := cfCertValidator.ValidateCertAliases(aliases, []string{*cdnCert}); err != nil {
				return fmt.Errorf("validate aliases against the imported CDN certificate for env %s: %w", d.env.Name, err)
			}
		}
		return nil
	}
	if d.app.Domain != "" {
		if err := validateAppVersionForAlias(d.app.Name, d.appVersionGetter); err != nil {
			logAppVersionOutdatedError(aws.StringValue(d.lbMft.Name))
			return err
		}
		return validateLBWSAlias(d.lbMft.RoutingRule.Alias, d.app, d.env.Name)
	}
	log.Errorf(ecsALBAliasUsedWithoutDomainFriendlyText)
	return fmt.Errorf("cannot specify http.alias when application is not associated with a domain and env %s doesn't import one or more certificates", d.env.Name)
}

func (d *lbWebSvcDeployer) validateNLBRuntime() error {
	if d.lbMft.NLBConfig.Aliases.IsEmpty() {
		return nil
	}

	hasImportedCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	if hasImportedCerts {
		return fmt.Errorf("cannot specify nlb.alias when env %s imports one or more certificates", d.env.Name)
	}
	if d.app.Domain == "" {
		log.Errorf(ecsNLBAliasUsedWithoutDomainFriendlyText)
		return fmt.Errorf("cannot specify nlb.alias when application is not associated with a domain")
	}
	if err := validateAppVersionForAlias(d.app.Name, d.appVersionGetter); err != nil {
		logAppVersionOutdatedError(aws.StringValue(d.lbMft.Name))
		return err
	}
	return validateLBWSAlias(d.lbMft.NLBConfig.Aliases, d.app, d.env.Name)
}

func validateLBWSAlias(aliases manifest.Alias, app *config.Application, envName string) error {
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
