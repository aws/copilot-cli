// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"github.com/aws/copilot-cli/internal/pkg/template/diff"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
)

const (
	fmtForceUpdateSvcStart    = "Forcing an update for service %s from environment %s"
	fmtForceUpdateSvcFailed   = "Failed to force an update for service %s from environment %s: %v.\n"
	fmtForceUpdateSvcComplete = "Forced an update for service %s from environment %s.\n"
)
const (
	imageTagLatest = "latest"
)

const (
	labelForBuilder       = "com.aws.copilot.image.builder"
	labelForVersion       = "com.aws.copilot.image.version"
	labelForContainerName = "com.aws.copilot.image.container.name"
)

// ActionRecommender contains methods that output action recommendation.
type ActionRecommender interface {
	RecommendedActions() []string
}

type noopActionRecommender struct{}

func (noopActionRecommender) RecommendedActions() []string {
	return nil
}

type imageBuilderPusher interface {
	BuildAndPush(docker repository.ContainerLoginBuildPusher, args *dockerengine.BuildArguments) (string, error)
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

type serviceDeployer interface {
	DeployService(conf cloudformation.StackConfiguration, bucketName string, opts ...awscloudformation.StackOption) error
}

type deployedTemplateGetter interface {
	Template(stackName string) (string, error)
}

type spinner interface {
	Start(label string)
	Stop(label string)
}

type fileReader interface {
	ReadFile(string) ([]byte, error)
}

// StackRuntimeConfiguration contains runtime configuration for a workload CloudFormation stack.
type StackRuntimeConfiguration struct {
	ImageDigests       map[string]ContainerImageIdentifier // Container name to image.
	EnvFileARNs        map[string]string
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

// GenerateCloudFormationTemplateInput is the input of GenerateCloudFormationTemplate.
type GenerateCloudFormationTemplateInput struct {
	StackRuntimeConfiguration
}

// GenerateCloudFormationTemplateOutput is the output of GenerateCloudFormationTemplate.
type GenerateCloudFormationTemplateOutput struct {
	Template   string
	Parameters string
}

type workloadDeployer struct {
	name          string
	app           *config.Application
	env           *config.Environment
	image         ContainerImageIdentifier
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
	tmplGetter         deployedTemplateGetter
	endpointGetter     endpointGetter
	spinner            spinner
	templateFS         template.Reader
	envVersionGetter   versionGetter
	overrider          Overrider
	customResources    customResourcesFunc

	// Cached variables.
	defaultSess              *session.Session
	defaultSessWithEnvRegion *session.Session
	envSess                  *session.Session
	store                    *config.Store
	envConfig                *manifest.Environment
}

// WorkloadDeployerInput is the input to for workloadDeployer constructor.
type WorkloadDeployerInput struct {
	SessionProvider  *sessions.Provider
	Name             string
	App              *config.Application
	Env              *config.Environment
	Image            ContainerImageIdentifier
	Mft              interface{} // Interpolated, applied, and unmarshaled manifest.
	RawMft           []byte      // Content of the manifest file without any transformations.
	EnvVersionGetter versionGetter
	Overrider        Overrider

	// Workload specific configuration.
	customResources customResourcesFunc
}

// ContainerImageIdentifier is the configuration of the image digest and tags of an ECR image.
type ContainerImageIdentifier struct {
	Digest            string
	CustomTag         string
	GitShortCommitTag string
}

// newWorkloadDeployer is the constructor for workloadDeployer.
func newWorkloadDeployer(in *WorkloadDeployerInput) (*workloadDeployer, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	defaultSession, err := in.SessionProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("create default: %w", err)
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
	addons, err = addon.ParseFromWorkload(in.Name, ws)
	if err != nil {
		var notFoundErr *addon.ErrAddonsNotFound
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("parse addons stack for workload %s: %w", in.Name, err)
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

	cfn := cloudformation.New(envSession, cloudformation.WithProgressTracker(os.Stderr))
	return &workloadDeployer{
		name:               in.Name,
		app:                in.App,
		env:                in.Env,
		image:              in.Image,
		resources:          resources,
		workspacePath:      ws.Path(),
		fs:                 &afero.Afero{Fs: afero.NewOsFs()},
		s3Client:           s3.New(envSession),
		addons:             addons,
		imageBuilderPusher: imageBuilderPusher,
		deployer:           cfn,
		tmplGetter:         cfn,
		endpointGetter:     envDescriber,
		spinner:            termprogress.NewSpinner(log.DiagnosticWriter),
		templateFS:         template.New(),
		envVersionGetter:   in.EnvVersionGetter,
		overrider:          in.Overrider,
		customResources:    in.customResources,

		defaultSess:              defaultSession,
		defaultSessWithEnvRegion: defaultSessEnvRegion,
		envSess:                  envSession,
		store:                    store,
		envConfig:                envConfig,

		mft:    in.Mft,
		rawMft: in.RawMft,
	}, nil
}

// DeployDiff returns the stringified diff of the template against the deployed template of the workload.
func (d *workloadDeployer) DeployDiff(template string) (string, error) {
	tmpl, err := d.tmplGetter.Template(stack.NameForWorkload(d.app.Name, d.env.Name, d.name))
	if err != nil {
		return "", fmt.Errorf("retrieve the deployed template for %q: %w", d.name, err)
	}
	diffTree, err := diff.From(tmpl).ParseWithCFNIgnorer([]byte(template))
	if err != nil {
		return "", fmt.Errorf("parse the diff against the deployed %q in environment %q: %w", d.name, d.env.Name, err)
	}
	buf := strings.Builder{}
	if err := diffTree.Write(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// AddonsTemplate returns this workload's addon template.
func (d *workloadDeployer) AddonsTemplate() (string, error) {
	if d.addons == nil {
		return "", nil
	}
	return d.addons.Template()
}

func (d *workloadDeployer) generateCloudFormationTemplate(conf stackSerializer) (
	*GenerateCloudFormationTemplateOutput, error) {
	tpl, err := conf.Template()
	if err != nil {
		return nil, err
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

// Tag returns the tag that should be used to reference the image.
// If the user provided their own tag then just use that
// Otherwise, return git short commit id.
func (img ContainerImageIdentifier) Tag() string {
	if img.CustomTag != "" {
		return img.CustomTag
	}
	return img.GitShortCommitTag
}

func (d *workloadDeployer) uploadContainerImages(out *UploadArtifactsOutput) error {
	// If it is built from local Dockerfile, build and push to the ECR repo.
	buildArgsPerContainer, err := buildArgsPerContainer(d.name, d.workspacePath, d.image, d.mft)
	if err != nil {
		return err
	}
	if len(buildArgsPerContainer) == 0 {
		return nil
	}
	out.ImageDigests = make(map[string]ContainerImageIdentifier, len(buildArgsPerContainer))
	for name, buildArgs := range buildArgsPerContainer {
		digest, err := d.imageBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArgs)
		if err != nil {
			return fmt.Errorf("build and push image: %w", err)
		}
		out.ImageDigests[name] = ContainerImageIdentifier{
			Digest:            digest,
			CustomTag:         d.image.CustomTag,
			GitShortCommitTag: d.image.GitShortCommitTag,
		}
	}
	return nil
}

func buildArgsPerContainer(name, workspacePath string, img ContainerImageIdentifier, unmarshaledManifest interface{}) (map[string]*dockerengine.BuildArguments, error) {
	type dfArgs interface {
		BuildArgs(rootDirectory string) (map[string]*manifest.DockerBuildArgs, error)
		ContainerPlatform() string
	}
	mf, ok := unmarshaledManifest.(dfArgs)
	if !ok {
		return nil, fmt.Errorf("%T does not have required methods BuildArgs() and ContainerPlatform()", name)
	}
	argsPerContainer, err := mf.BuildArgs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("check if manifest requires building from local Dockerfile: %w", err)
	}
	dArgs := make(map[string]*dockerengine.BuildArguments, len(argsPerContainer))
	for container, buildArgs := range argsPerContainer {
		tags := []string{imageTagLatest}
		if img.Tag() != "" {
			tags = append(tags, img.Tag())
		}
		if container != name {
			tags = []string{fmt.Sprintf("%s-%s", container, imageTagLatest)}
			if img.GitShortCommitTag != "" {
				tags = append(tags, fmt.Sprintf("%s-%s", container, img.GitShortCommitTag))
			}
		}
		labels := make(map[string]string, 3)
		labels[labelForBuilder] = "copilot-cli"
		if version.Version != "" {
			labels[labelForVersion] = version.Version
		}
		labels[labelForContainerName] = container
		dArgs[container] = &dockerengine.BuildArguments{
			Dockerfile: aws.StringValue(buildArgs.Dockerfile),
			Context:    aws.StringValue(buildArgs.Context),
			Args:       buildArgs.Args,
			CacheFrom:  buildArgs.CacheFrom,
			Target:     aws.StringValue(buildArgs.Target),
			Platform:   mf.ContainerPlatform(),
			Tags:       tags,
			Labels:     labels,
		}
	}
	return dArgs, nil
}

func (d *workloadDeployer) uploadArtifactsToS3(out *UploadArtifactsOutput) error {
	var err error
	out.EnvFileARNs, err = d.pushEnvFilesToS3Bucket(&pushEnvFilesToS3BucketInput{
		fs:       d.fs,
		uploader: d.s3Client,
	})
	if err != nil {
		return err
	}
	out.AddonsURL, err = d.pushAddonsTemplateToS3Bucket()
	if err != nil {
		return err
	}
	return nil
}

type customResourcesFunc func(fs template.Reader) ([]*customresource.CustomResource, error)

// UploadArtifactsOutput is the output of UploadArtifacts.
type UploadArtifactsOutput struct {
	ImageDigests       map[string]ContainerImageIdentifier // Container name to image.
	EnvFileARNs        map[string]string                   // map[container name]envFileARN
	AddonsURL          string
	CustomResourceURLs map[string]string
}

// uploadArtifactFunc uploads an artifact and updates out
// with any relevant information to be returned by uploadArtifacts.
type uploadArtifactFunc func(out *UploadArtifactsOutput) error

// uploadArtifacts runs each of the uploadArtifact functions sequentially and returns
// the output built by each of those functions. It short-circuts and returns
// the error if one of steps returns an error.
func (d *workloadDeployer) uploadArtifacts(steps ...uploadArtifactFunc) (*UploadArtifactsOutput, error) {
	out := &UploadArtifactsOutput{}
	for _, step := range steps {
		if err := step(out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (d *workloadDeployer) uploadCustomResources(out *UploadArtifactsOutput) error {
	crs, err := d.customResources(d.templateFS)
	if err != nil {
		return err
	}
	urls, err := customresource.Upload(func(key string, contents io.Reader) (string, error) {
		return d.s3Client.Upload(d.resources.S3Bucket, key, contents)
	}, crs)
	if err != nil {
		return fmt.Errorf("upload custom resources for %q: %w", d.name, err)
	}
	out.CustomResourceURLs = urls
	return nil
}

type pushEnvFilesToS3BucketInput struct {
	fs       fileReader
	uploader uploader
}

// pushEnvFilesToS3Bucket collects all environment files required by the workload container and any sidecars,
// uploads each one exactly once, and returns a map structured in the following way. map[containerName]envFileARN
// Example content:
//
//	{
//	  "frontend": "arn:aws:s3:::bucket/key1",
//	  "firelens_log_router": "arn:aws:s3:::bucket/key2",
//	  "nginx": "arn:aws:s3:::bucket/key1"
//	}
func (d *workloadDeployer) pushEnvFilesToS3Bucket(in *pushEnvFilesToS3BucketInput) (map[string]string, error) {
	envFilesByContainer := envFiles(d.mft)
	uniqueEnvFiles := make(map[string][]string)
	// Invert the map of containers to env files to get the unique env files to upload.
	for container, path := range envFilesByContainer {
		if path == "" {
			continue
		}
		if containers, ok := uniqueEnvFiles[path]; !ok {
			uniqueEnvFiles[path] = []string{container}
		} else {
			uniqueEnvFiles[path] = append(containers, container)
		}
	}

	if len(uniqueEnvFiles) == 0 {
		return nil, nil
	}
	// Upload each file to s3 exactly once and generate its ARN.
	// Save those ARNs in a map from container name to env file ARN and return.
	envFileARNs := make(map[string]string)

	for path, containers := range uniqueEnvFiles {
		content, err := in.fs.ReadFile(filepath.Join(d.workspacePath, path))
		if err != nil {
			return nil, fmt.Errorf("read env file %s: %w", path, err)
		}
		reader := bytes.NewReader(content)
		url, err := in.uploader.Upload(d.resources.S3Bucket, artifactpath.EnvFiles(path, content), reader)
		if err != nil {
			return nil, fmt.Errorf("put env file %s artifact to bucket %s: %w", path, d.resources.S3Bucket, err)
		}
		bucket, key, err := s3.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("parse s3 url: %w", err)
		}
		// The app and environment are always within the same partition.
		partition, err := partitions.Region(d.env.Region).Partition()
		if err != nil {
			return nil, err
		}
		for _, container := range containers {
			envFileARNs[container] = s3.FormatARN(partition.ID(), fmt.Sprintf("%s/%s", bucket, key))
		}
	}
	return envFileARNs, nil
}

// envFiles gets a map from container name to env file for all containers in the task,
// including sidecars and Firelens logging.
func envFiles(unmarshaledManifest interface{}) map[string]string {
	type envFile interface {
		EnvFiles() map[string]string
	}
	mf, ok := unmarshaledManifest.(envFile)
	if ok {
		return mf.EnvFiles()
	}
	// If the manifest type doesn't support envFiles, ignore and move forward.
	return nil
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
	envVersion, err := d.envVersionGetter.Version()
	if err != nil {
		return nil, fmt.Errorf("get version of environment %q: %w", d.env.Name, err)
	}
	if len(in.ImageDigests) == 0 {
		return &stack.RuntimeConfig{
			AddonsTemplateURL:        in.AddonsURL,
			EnvFileARNs:              in.EnvFileARNs,
			AdditionalTags:           in.Tags,
			ServiceDiscoveryEndpoint: endpoint,
			AccountID:                d.env.AccountID,
			Region:                   d.env.Region,
			CustomResourcesURL:       in.CustomResourceURLs,
			EnvVersion:               envVersion,
		}, nil
	}
	images := make(map[string]stack.ECRImage, len(in.ImageDigests))
	for container, img := range in.ImageDigests {
		// Currently we do not tag sidecar container images with custom tag provided by the user.
		// This is the reason for having different ImageTag for main and sidecar container images
		// that is needed to create CloudFormation stack.
		imageTag := img.Tag()
		if container != d.name {
			imageTag = img.GitShortCommitTag
		}
		images[container] = stack.ECRImage{
			RepoURL:           d.resources.RepositoryURLs[d.name],
			ImageTag:          imageTag,
			Digest:            img.Digest,
			MainContainerName: d.name,
			ContainerName:     container,
		}
	}
	return &stack.RuntimeConfig{
		AddonsTemplateURL:        in.AddonsURL,
		EnvFileARNs:              in.EnvFileARNs,
		AdditionalTags:           in.Tags,
		PushedImages:             images,
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                d.env.AccountID,
		Region:                   d.env.Region,
		CustomResourcesURL:       in.CustomResourceURLs,
		EnvVersion:               envVersion,
	}, nil
}

type timeoutError interface {
	error
	Timeout() bool
}
