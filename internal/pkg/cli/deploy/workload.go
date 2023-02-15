// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"bytes"
	"context"
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
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

const (
	fmtForceUpdateSvcStart    = "Forcing an update for service %s from environment %s"
	fmtForceUpdateSvcFailed   = "Failed to force an update for service %s from environment %s: %v.\n"
	fmtForceUpdateSvcComplete = "Forced an update for service %s from environment %s.\n"
)

// ActionRecommender contains methods that output action recommendation.
type ActionRecommender interface {
	RecommendedActions() []string
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

type spinner interface {
	Start(label string)
	Stop(label string)
}

type fileReader interface {
	ReadFile(string) ([]byte, error)
}

type dfArgs interface {
	SidecarBuildArgs(wsRoot string) map[string]*manifest.DockerBuildArgs
	ContainerPlatform() string
}

// StackRuntimeConfiguration contains runtime configuration for a workload CloudFormation stack.
type StackRuntimeConfiguration struct {
	// Use *string for three states (see https://github.com/aws/copilot-cli/pull/3268#discussion_r806060230)
	// This is mainly to keep the `workload package` behavior backward-compatible, otherwise our old pipeline buildspec would break,
	// since previously we parsed the env region from a mock ECR URL that we generated from `workload package``.
	ImageDigest        *string
	ScImageDigests     map[string]string
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
	imageTag      string
	gitTag        string
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
	ImageTag         string
	Mft              interface{} // Interpolated, applied, and unmarshaled manifest.
	RawMft           []byte      // Content of the manifest file without any transformations.
	EnvVersionGetter versionGetter
	Overrider        Overrider
	GitTag           string

	// Workload specific configuration.
	customResources customResourcesFunc
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

	return &workloadDeployer{
		name:               in.Name,
		app:                in.App,
		env:                in.Env,
		imageTag:           in.ImageTag,
		gitTag:             in.GitTag,
		resources:          resources,
		workspacePath:      ws.Path(),
		fs:                 &afero.Afero{Fs: afero.NewOsFs()},
		s3Client:           s3.New(envSession),
		addons:             addons,
		imageBuilderPusher: imageBuilderPusher,
		deployer:           cloudformation.New(envSession, cloudformation.WithProgressTracker(os.Stderr)),
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

// AddonsTemplate returns this workload's addon template.
func (w *workloadDeployer) AddonsTemplate() (string, error) {
	if w.addons == nil {
		return "", nil
	}

	return w.addons.Template()
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

func (d *workloadDeployer) uploadContainerImage(imgBuilderPusher imageBuilderPusher) (*string, map[string]string, error) {
	required, err := manifest.DockerfileBuildRequired(d.mft)
	if err != nil {
		return nil, nil, err
	}
	UUIDTag, err := generateUUID()
	if err != nil {
		return nil, nil, err
	}
	var digest string
	if required {
		// If it is built from local Dockerfile, build and push to the ECR repo.
		buildArg, err := buildArgs(d.name, d.imageTag, d.gitTag, UUIDTag, d.workspacePath, d.mft)
		if err != nil {
			return nil, nil, err
		}
		digest, err = imgBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), buildArg)
		if err != nil {
			return nil, nil, fmt.Errorf("build and push main container image: %w", err)
		}
	}
	args := scBuildArgs(d.name, d.gitTag, UUIDTag, d.workspacePath, d.mft)
	scDigests := make(map[string]string, len(args))
	ctx := context.Background()
	g, _ := errgroup.WithContext(ctx)
	for k, v := range args {
		scName := k
		dArgs := v
		g.Go(func() error {
			scDigests[scName], err = imgBuilderPusher.BuildAndPush(dockerengine.New(exec.NewCmd()), dArgs)
			if err != nil {
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	// The below if loop is for simple test cases.
	if !required {
		return nil, scDigests, nil
	}
	log.Infoln(scDigests)
	return aws.String(digest), scDigests, nil
}

// generateUUID generates a random UUID to tag main and sidecar containers.
func generateUUID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate random uuid to tag main and sidecar images: %w", err)
	}
	return id.String(), err
}

func buildArgs(name, imageTag, gitTag, uuid, workspacePath string, unmarshaledManifest interface{}) (*dockerengine.BuildArguments, error) {
	type dfArgs interface {
		BuildArgs(rootDirectory string) *manifest.DockerBuildArgs
		ContainerPlatform() string
	}
	mf, ok := unmarshaledManifest.(dfArgs)
	if !ok {
		return nil, fmt.Errorf("%s does not have required methods BuildArgs() and ContainerPlatform()", name)
	}
	args := mf.BuildArgs(workspacePath)
	return &dockerengine.BuildArguments{
		Dockerfile: *args.Dockerfile,
		Context:    *args.Context,
		Args:       args.Args,
		CacheFrom:  args.CacheFrom,
		Target:     aws.StringValue(args.Target),
		Platform:   mf.ContainerPlatform(),
		Tags:       mainContainerTags(imageTag, gitTag, uuid),
	}, nil
}

// mainContainerTags() returns tags for workload main container image.
// 1. If userTag is not provided and not in git repository then return "uuid" and "latest".
// 2. If userTag is not provided and in git repository then return "commitId" and "latest".
// 3. If userTag is provided and not in git repository then return "userTag" and "latest".
// 4. If userTag is provided and in git repository then return "userTag" and "latest".
func mainContainerTags(userTag, gitTag, uuid string) []string {
	var tags []string
	tags = append(tags, "latest")
	ugTag := userOrGitTag(userTag, gitTag)
	if ugTag != "" {
		tags = append(tags, ugTag)
	} else {
		tags = append(tags, uuid)
	}
	return tags
}

// userOrGitTag returns if the user provided their own tag.
// Otherwise, return git commit id.
func userOrGitTag(userTag, gitTag string) string {
	if userTag != "" {
		return userTag
	}
	return gitTag
}

// scBuildArgs returns the map of dockerengine build arguments for the each sidecar that requires a native build from dockerfile.
func scBuildArgs(name, gitTag, uuid, workspacePath string, unmarshaledManifest interface{}) map[string]*dockerengine.BuildArguments {
	mf, ok := unmarshaledManifest.(dfArgs)
	var args map[string]*manifest.DockerBuildArgs
	if ok {
		args = mf.SidecarBuildArgs(workspacePath)
	}
	if args == nil {
		return nil
	}
	deBuildArgs := make(map[string]*dockerengine.BuildArguments, len(args))
	for k, v := range args {
		deBuildArgs[k] = &dockerengine.BuildArguments{
			Dockerfile: aws.StringValue(v.Dockerfile),
			Context:    aws.StringValue(v.Context),
			Args:       v.Args,
			CacheFrom:  v.CacheFrom,
			Target:     aws.StringValue(v.Target),
			Platform:   mf.ContainerPlatform(),
			Tags:       scContainerTags(k, gitTag, uuid),
		}
	}
	return deBuildArgs
}

// scContainerTags() returns tags for sidecar container images in a workload.
// 1. If user is not in git repository then return "sidecarName-latest" and "sidecarName-uuid".
// 2. If user is in git repository then return "sidecarName-latest" and "sidecarName-gitCommitId".
func scContainerTags(scName, gitTag, uuid string) []string {
	var tags []string
	defaultScTag := fmt.Sprintf("%s-%s", scName, "latest")
	tags = append(tags, defaultScTag)
	if gitTag != "" {
		tags = append(tags, fmt.Sprintf("%s-%s", scName, gitTag))
	} else {
		tags = append(tags, fmt.Sprintf("%s-%s", scName, uuid))
	}
	return tags
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

type customResourcesFunc func(fs template.Reader) ([]*customresource.CustomResource, error)

// UploadArtifactsOutput is the output of UploadArtifacts.
type UploadArtifactsOutput struct {
	ImageDigest        *string
	ScImageDigests     map[string]string
	EnvFileARN         string
	AddonsURL          string
	CustomResourceURLs map[string]string
}

func (d *workloadDeployer) uploadArtifacts() (*UploadArtifactsOutput, error) {
	imageDigest, scImageDigests, err := d.uploadContainerImage(d.imageBuilderPusher)
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
		ImageDigest:    imageDigest,
		ScImageDigests: scImageDigests,
		EnvFileARN:     s3Artifacts.envFileARN,
		AddonsURL:      s3Artifacts.addonsURL,
	}
	crs, err := d.customResources(d.templateFS)
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
	if len(aws.StringValue(in.ImageDigest)) == 0 {
		return &stack.RuntimeConfig{
			AddonsTemplateURL:        in.AddonsURL,
			EnvFileARN:               in.EnvFileARN,
			AdditionalTags:           in.Tags,
			ServiceDiscoveryEndpoint: endpoint,
			AccountID:                d.env.AccountID,
			Region:                   d.env.Region,
			CustomResourcesURL:       in.CustomResourceURLs,
			EnvVersion:               envVersion,
			ScECRImage: stack.SidecarECRImage{
				RepoURL:             d.resources.RepositoryURLs[d.name],
				SidecarImageTag:     d.gitTag,
				SidecarImageDigests: in.ScImageDigests,
			},
		}, nil
	}
	return &stack.RuntimeConfig{
		AddonsTemplateURL: in.AddonsURL,
		EnvFileARN:        in.EnvFileARN,
		AdditionalTags:    in.Tags,
		Image: &stack.ECRImage{
			RepoURL:  d.resources.RepositoryURLs[d.name],
			ImageTag: userOrGitTag(d.imageTag, d.gitTag),
			Digest:   aws.StringValue(in.ImageDigest),
		},
		ServiceDiscoveryEndpoint: endpoint,
		AccountID:                d.env.AccountID,
		Region:                   d.env.Region,
		CustomResourcesURL:       in.CustomResourceURLs,
		EnvVersion:               envVersion,
		ScECRImage: stack.SidecarECRImage{
			RepoURL:             d.resources.RepositoryURLs[d.name],
			SidecarImageTag:     d.gitTag,
			SidecarImageDigests: in.ScImageDigests,
		},
	}, nil
}

type timeoutError interface {
	error
	Timeout() bool
}
