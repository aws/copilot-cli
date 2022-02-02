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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/apprunner"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
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
	"github.com/aws/copilot-cli/internal/pkg/workspace"
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

// ImageBuilderPusher builds and pushes an image.
type ImageBuilderPusher interface {
	BuildAndPush(docker repository.ContainerLoginBuildPusher, args *dockerengine.BuildArguments) (string, error)
}

// Uploader uploads a file.
type Uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

// Templater stringifies a golang template.
type Templater interface {
	Template() (string, error)
}

// EndpointGetter gets the service discovery endpoint.
type EndpointGetter interface {
	ServiceDiscoveryEndpoint() (string, error)
}

// VersionGetter gets the version.
type VersionGetter interface {
	Version() (string, error)
}

// PublicCIDRBlocksGetter gets the public CIDR blocks.
type PublicCIDRBlocksGetter interface {
	PublicCIDRBlocks() ([]string, error)
}

// CustomResourcesUploader uploads the custom resource files to S3.
type CustomResourcesUploader interface {
	UploadEnvironmentCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
	UploadRequestDrivenWebServiceCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
	UploadNetworkLoadBalancedWebServiceCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error)
}

// SNSTopicsLister lists SNS topics.
type SNSTopicsLister interface {
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

// ServiceDeployer uses CloudFormation to deploy a service.
type ServiceDeployer interface {
	DeployService(out progress.FileWriter, conf cloudformation.StackConfiguration, bucketName string, opts ...awscloudformation.StackOption) error
}

// ServiceForceUpdater force updates a service.
type ServiceForceUpdater interface {
	ForceUpdateService(app, env, svc string) error
	LastUpdatedAt(app, env, svc string) (time.Time, error)
}

// WorkspaceReader reads from the workspace.
type WorkspaceReader interface {
	manifestReader
	pathFinder
}

// Interpolator interpolates a manifest.
type Interpolator interface {
	Interpolate(s string) (string, error)
}

// progress is the interface to inform the user that a long operation is taking place.
type Progress interface {
	Start(label string)
	Stop(label string)
}

type manifestReader interface {
	ReadWorkloadManifest(name string) (workspace.WorkloadManifest, error)
}

type pathFinder interface {
	Path() (string, error)
}

type fileReader interface {
	ReadFile(string) ([]byte, error)
}

type workloadDeployer struct {
	name          string
	app           *config.Application
	env           *config.Environment
	imageTag      string
	s3Bucket      string
	mft           interface{}
	workspacePath string
}

// WorkloadDeployerInput is the input to for workloadDeployer constructor.
type WorkloadDeployerInput struct {
	Name     string
	App      *config.Application
	Env      *config.Environment
	ImageTag string
	S3Bucket string
}

// NewWorkloadDeployer is the constructor for workloadDeployer.
func NewWorkloadDeployer(in *WorkloadDeployerInput) (*workloadDeployer, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	mft, err := workloadManifest(&workloadManifestInput{
		name:         in.Name,
		appName:      in.App.Name,
		envName:      in.Env.Name,
		interpolator: manifest.NewInterpolator(in.App.Name, in.Env.Name),
		ws:           ws,
		unmarshal:    manifest.UnmarshalWorkload,
	})
	if err != nil {
		return nil, err
	}
	workspacePath, err := ws.Path()
	if err != nil {
		return nil, fmt.Errorf("get workspace path: %w", err)
	}
	return &workloadDeployer{
		name:          in.Name,
		app:           in.App,
		env:           in.Env,
		imageTag:      in.ImageTag,
		s3Bucket:      in.S3Bucket,
		mft:           mft,
		workspacePath: workspacePath,
	}, nil
}

// UploadArtifactsInput is the input of UploadArtifacts.
type UploadArtifactsInput struct {
	FS                 fileReader
	Uploader           Uploader
	Templater          Templater
	ImageBuilderPusher ImageBuilderPusher
}

// UploadArtifactsOutput is the output of UploadArtifacts.
type UploadArtifactsOutput struct {
	ImageDigest string
	EnvFileARN  string
	AddonsURL   string
}

// DeployWorkloadInput is the input of DeployWorkload.
type DeployWorkloadInput struct {
	ImageDigest    string
	EnvFileARN     string
	AddonsURL      string
	RootUserARN    string
	Tags           map[string]string
	ImageRepoURL   string
	ForceNewUpdate bool

	CustomResourceUploader CustomResourcesUploader
	S3Uploader             Uploader
	SNSTopicsLister        SNSTopicsLister
	ServiceDeployer        ServiceDeployer
	NewSvcUpdater          func(func(*session.Session) ServiceForceUpdater) ServiceForceUpdater
	AppVersionGetter       VersionGetter
	PublicCIDRBlocksGetter PublicCIDRBlocksGetter
	EndpointGetter         EndpointGetter
	Spinner                Progress

	Now func() time.Time
}

// DeployWorkloadOutput is the output of DeployWorkload.
type DeployWorkloadOutput struct {
	AppliedMft    interface{}
	RDWSAlias     string
	Subscriptions []manifest.TopicSubscription
}

// UploadArtifacts uploads the deployment artifacts (image, addons files, env files).
func (d *workloadDeployer) UploadArtifacts(in *UploadArtifactsInput) (*UploadArtifactsOutput, error) {
	imageOutput, err := d.uploadContainerImage(in.ImageBuilderPusher)
	if err != nil {
		return nil, err
	}
	s3Artifacts, err := d.uploadArtifactsToS3(&uploadArtifactsToS3Input{
		fs:        in.FS,
		uploader:  in.Uploader,
		templater: in.Templater,
	})
	if err != nil {
		return nil, err
	}

	return &UploadArtifactsOutput{
		ImageDigest: imageOutput.imageDigest,
		EnvFileARN:  s3Artifacts.envFileARN,
		AddonsURL:   s3Artifacts.addonsURL,
	}, nil
}

// DeployWorkload deploys a workload using CloudFormation.
func (d *workloadDeployer) DeployWorkload(in *DeployWorkloadInput) (*DeployWorkloadOutput, error) {
	stackConfigOutput, err := d.stackConfiguration(in)
	if err != nil {
		return nil, err
	}
	cmdRunAt := in.Now()
	if err := in.ServiceDeployer.DeployService(os.Stderr, stackConfigOutput.conf, d.s3Bucket,
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN)); err != nil {
		var errEmptyCS *awscloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyCS) {
			return nil, fmt.Errorf("deploy service: %w", err)
		}
		if !in.ForceNewUpdate {
			log.Warningln("Set --force to force an update for the service.")
			return nil, fmt.Errorf("deploy service: %w", err)
		}
	}
	// Force update the service if --force is set and the service is not updated by the CFN.
	if in.ForceNewUpdate {
		lastUpdatedAt, err := stackConfigOutput.svcUpdater.LastUpdatedAt(d.app.Name, d.env.Name, d.name)
		if err != nil {
			return nil, fmt.Errorf("get the last updated deployment time for %s: %w", d.name, err)
		}
		if cmdRunAt.After(lastUpdatedAt) {
			if err := d.forceDeploy(&forceDeployInput{
				spinner:    in.Spinner,
				svcUpdater: stackConfigOutput.svcUpdater,
			}); err != nil {
				return nil, err
			}
		}
	}
	return &DeployWorkloadOutput{
		RDWSAlias:     stackConfigOutput.rdSvcAlias,
		Subscriptions: stackConfigOutput.subscriptions,
		AppliedMft:    d.mft,
	}, nil
}

type forceDeployInput struct {
	spinner    Progress
	svcUpdater ServiceForceUpdater
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

type uploadContainerImageOutput struct {
	buildRequired bool
	imageDigest   string
}

func (d *workloadDeployer) uploadContainerImage(imgBuilderPusher ImageBuilderPusher) (
	uploadContainerImageOutput, error) {
	required, err := manifest.DockerfileBuildRequired(d.mft)
	if err != nil {
		return uploadContainerImageOutput{}, err
	}
	if !required {
		return uploadContainerImageOutput{}, nil
	}
	// If it is built from local Dockerfile, build and push to the ECR repo.
	buildArg, err := buildArgs(d.name, d.imageTag, d.workspacePath, d.mft)
	if err != nil {
		return uploadContainerImageOutput{}, err
	}
	digest, err := imgBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArg)
	if err != nil {
		return uploadContainerImageOutput{}, fmt.Errorf("build and push image: %w", err)
	}
	return uploadContainerImageOutput{
		buildRequired: true,
		imageDigest:   digest,
	}, nil
}

type uploadArtifactsToS3Input struct {
	fs        fileReader
	uploader  Uploader
	templater Templater
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
	uploader Uploader
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
	url, err := in.uploader.Upload(d.s3Bucket, s3.MkdirSHA256(path, content), reader)
	if err != nil {
		return "", fmt.Errorf("put env file %s artifact to bucket %s: %w", path, d.s3Bucket, err)
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
	templater Templater
	uploader  Uploader
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
	url, err := in.uploader.Upload(d.s3Bucket, fmt.Sprintf(deploy.AddonsCfnTemplateNameFormat, d.name), reader)
	if err != nil {
		return "", fmt.Errorf("put addons artifact to bucket %s: %w", d.s3Bucket, err)
	}
	return url, nil
}

func (d *workloadDeployer) runtimeConfig(in *DeployWorkloadInput) (*stack.RuntimeConfig, error) {
	endpoint, err := in.EndpointGetter.ServiceDiscoveryEndpoint()
	if err != nil {
		return nil, fmt.Errorf("get service discovery endpoint: %w", err)
	}

	if in.ImageDigest == "" {
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
			RepoURL:  in.ImageRepoURL,
			ImageTag: d.imageTag,
			Digest:   in.ImageDigest,
		},
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                d.env.AccountID,
		Region:                   d.env.Region,
	}, nil
}

type stackConfigurationOutput struct {
	conf          cloudformation.StackConfiguration
	rdSvcAlias    string
	subscriptions []manifest.TopicSubscription
	svcUpdater    ServiceForceUpdater
}

func (d *workloadDeployer) stackConfiguration(in *DeployWorkloadInput) (*stackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	var output stackConfigurationOutput
	switch t := d.mft.(type) {
	case *manifest.LoadBalancedWebService:
		if err := validateLBWSRuntime(d.app, d.env.Name, t, in.AppVersionGetter); err != nil {
			return nil, err
		}
		var opts []stack.LoadBalancedWebServiceOption
		if !t.NLBConfig.IsEmpty() {
			cidrBlocks, err := in.PublicCIDRBlocksGetter.PublicCIDRBlocks()
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

			if !t.RoutingRule.Disabled() {
				opts = append(opts, stack.WithHTTPS())
			}
		}
		output.svcUpdater = in.NewSvcUpdater(func(s *session.Session) ServiceForceUpdater {
			return ecs.New(s)
		})
		output.conf, err = stack.NewLoadBalancedWebService(t, d.env.Name, d.app.Name, *rc, opts...)
	case *manifest.RequestDrivenWebService:
		if d.app.Domain == "" && t.Alias != nil {
			log.Errorf(aliasUsedWithoutDomainFriendlyText)
			return nil, errors.New("alias specified when application is not associated with a domain")
		}
		appInfo := deploy.AppInformation{
			Name:                d.app.Name,
			DNSName:             d.app.Domain,
			AccountPrincipalARN: in.RootUserARN,
		}
		if t.Alias == nil {
			output.conf, err = stack.NewRequestDrivenWebService(t, d.env.Name, appInfo, *rc)
			break
		}

		output.rdSvcAlias = aws.StringValue(t.Alias)

		if err = validateRDSvcAliasAndAppVersion(d.name,
			aws.StringValue(t.Alias), d.env.Name, d.app, in.AppVersionGetter); err != nil {
			return nil, err
		}
		var urls map[string]string
		if urls, err = uploadRDWSCustomResources(&uploadRDWSCustomResourcesInput{
			customResourceUploader: in.CustomResourceUploader,
			s3Uploader:             in.S3Uploader,
			s3Bucket:               d.s3Bucket,
		}); err != nil {
			return nil, err
		}
		output.svcUpdater = in.NewSvcUpdater(func(s *session.Session) ServiceForceUpdater {
			return apprunner.New(s)
		})
		output.conf, err = stack.NewRequestDrivenWebServiceWithAlias(t, d.env.Name, appInfo, *rc, urls)
	case *manifest.BackendService:
		output.svcUpdater = in.NewSvcUpdater(func(s *session.Session) ServiceForceUpdater {
			return ecs.New(s)
		})
		output.conf, err = stack.NewBackendService(t, d.env.Name, d.app.Name, *rc)
	case *manifest.WorkerService:
		var topics []deploy.Topic
		topics, err = in.SNSTopicsLister.ListSNSTopics(d.app.Name, d.env.Name)
		if err != nil {
			return nil, fmt.Errorf("get SNS topics for app %s and environment %s: %w", d.app.Name, d.env.Name, err)
		}
		var topicARNs []string
		for _, topic := range topics {
			topicARNs = append(topicARNs, topic.ARN())
		}
		type subscriptions interface {
			Subscriptions() []manifest.TopicSubscription
		}

		subscriptionGetter, ok := d.mft.(subscriptions)
		if !ok {
			return nil, errors.New("manifest does not have required method Subscriptions")
		}
		// Cache the subscriptions for later.
		output.subscriptions = subscriptionGetter.Subscriptions()

		if err = validateTopicsExist(output.subscriptions, topicARNs, d.app.Name, d.env.Name); err != nil {
			return nil, err
		}
		output.svcUpdater = in.NewSvcUpdater(func(s *session.Session) ServiceForceUpdater {
			return ecs.New(s)
		})
		output.conf, err = stack.NewWorkerService(t, d.env.Name, d.app.Name, *rc)
	case *manifest.ScheduledJob:
		output.conf, err = stack.NewScheduledJob(t, d.env.Name, d.app.Name, *rc)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return &output, nil
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

type workloadManifestInput struct {
	name         string
	appName      string
	envName      string
	ws           WorkspaceReader
	interpolator Interpolator
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
	customResourceUploader CustomResourcesUploader
	s3Uploader             Uploader
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

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter VersionGetter) error {
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

func validateAppVersion(appName string, appVersionGetter VersionGetter) error {
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

func validateLBWSRuntime(app *config.Application, envName string, mft *manifest.LoadBalancedWebService, appVersionGetter VersionGetter) error {
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
