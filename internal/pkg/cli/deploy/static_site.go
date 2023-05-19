// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"io"
	"path/filepath"

	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/asset"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
)

const artifactBucketAssetsDir = "local-assets"

type fileUploader interface {
	UploadFiles(files []manifest.FileUpload) (string, error)
}

type rootGetter interface {
	ProjectRoot() string
}

type staticSiteDeployer struct {
	*svcDeployer
	appVersionGetter versionGetter
	staticSiteMft    *manifest.StaticSite
	fs               afero.Fs
	uploader         fileUploader
	rootGetter       rootGetter
	newStack         func(*stack.StaticSiteConfig) (*stack.StaticSite, error)
}

// NewStaticSiteDeployer is the constructor for staticSiteDeployer.
func NewStaticSiteDeployer(in *WorkloadDeployerInput) (*staticSiteDeployer, error) {
	in.customResources = staticSiteCustomResources
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	versionGetter, err := describe.NewAppDescriber(in.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new app describer for application %s: %w", in.App.Name, err)
	}
	mft, ok := in.Mft.(*manifest.StaticSite)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.StaticSiteType)
	}
	ws, err := workspace.Use(svcDeployer.fs)
	if err != nil {
		return nil, err
	}
	return &staticSiteDeployer{
		svcDeployer:      svcDeployer,
		appVersionGetter: versionGetter,
		staticSiteMft:    mft,
		fs:               svcDeployer.fs,
		uploader: &asset.ArtifactBucketUploader{
			FS:                  svcDeployer.fs,
			AssetDir:            artifactBucketAssetsDir,
			AssetMappingFileDir: fmt.Sprintf("%s/environments/%s/workloads/%s/mapping", artifactBucketAssetsDir, svcDeployer.env.Name, svcDeployer.name),
			Upload: func(path string, data io.Reader) error {
				_, err := svcDeployer.s3Client.Upload(svcDeployer.resources.S3Bucket, path, data)
				return err
			},
		},
		rootGetter: ws,
		newStack:   stack.NewStaticSite,
	}, nil
}

func staticSiteCustomResources(fs template.Reader) ([]*customresource.CustomResource, error) {
	crs, err := customresource.StaticSite(fs)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for a %q: %w", manifestinfo.StaticSiteType, err)
	}
	return crs, nil
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (*staticSiteDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(s3.EndpointsID, region)
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *staticSiteDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	conf, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(conf)
}

// DeployWorkload deploys a static site service using CloudFormation.
func (d *staticSiteDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	conf, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.Options, svcStackConfigurationOutput{
		conf: cloudformation.WrapWithTemplateOverrider(conf, d.overrider),
	}); err != nil {
		return nil, err
	}
	return noopActionRecommender{}, nil
}

func (d *staticSiteDeployer) deploy(deployOptions Options, stackConfigOutput svcStackConfigurationOutput) error {
	opts := []awscloudformation.StackOption{
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN),
	}
	if deployOptions.DisableRollback {
		opts = append(opts, awscloudformation.WithDisableRollback())
	}
	if err := d.deployer.DeployService(stackConfigOutput.conf, d.resources.S3Bucket, opts...); err != nil {
		return fmt.Errorf("deploy service: %w", err)
	}
	return nil
}

// UploadArtifacts uploads static assets to the app stackset bucket.
func (d *staticSiteDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.uploadStaticFiles, d.uploadArtifactsToS3, d.uploadCustomResources)
}

func (d *staticSiteDeployer) uploadStaticFiles(out *UploadArtifactsOutput) error {
	fullPathSources, err := d.convertSources()
	if err != nil {
		return err
	}
	path, err := d.uploader.UploadFiles(fullPathSources)
	if err != nil {
		return fmt.Errorf("upload static files: %w", err)
	}

	out.StaticSiteAssetMappingLocation = s3.Location(d.resources.S3Bucket, path)
	return nil
}

func (d *staticSiteDeployer) stackConfiguration(in *StackRuntimeConfiguration) (cloudformation.StackConfiguration, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	if err := d.validateAlias(); err != nil {
		return nil, err
	}
	if err := validateMinAppVersion(d.app.Name, d.name, d.appVersionGetter, deploy.StaticSiteMinAppTemplateVersion); err != nil {
		return nil, fmt.Errorf("static sites not supported: %w", err)
	}
	conf, err := d.newStack(&stack.StaticSiteConfig{
		App:                d.app,
		EnvManifest:        d.envConfig,
		Manifest:           d.staticSiteMft,
		RawManifest:        d.rawMft,
		ArtifactBucketName: d.resources.S3Bucket,
		RuntimeConfig:      *rc,
		RootUserARN:        in.RootUserARN,
		Addons:             d.addons,
		AssetMappingURL:    in.StaticSiteAssetMappingURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}
	return conf, nil
}

// convertSources transforms the source's path relative to the project root into the full path.
func (d *staticSiteDeployer) convertSources() ([]manifest.FileUpload, error) {
	var convertedFileUploads = make([]manifest.FileUpload, len(d.staticSiteMft.FileUploads))
	for i, source := range d.staticSiteMft.FileUploads {
		root := d.rootGetter.ProjectRoot()
		fullPath := filepath.Join(root, source.Source)
		fileInfo, err := d.fs.Stat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("source %q must be a valid path relative to the workspace root %q: %w", source.Source, root, err)
		}
		convertedFileUploads[i] = manifest.FileUpload{
			Source:      fullPath,
			Destination: source.Destination,
			Recursive:   fileInfo.IsDir(),
			Exclude:     source.Exclude,
			Reinclude:   source.Reinclude,
		}

	}
	return convertedFileUploads, nil
}

func (d *staticSiteDeployer) validateAlias() error {
	if d.staticSiteMft.HTTP.Alias == "" {
		return nil
	}

	hasImportedCerts := len(d.envConfig.HTTPConfig.Public.Certificates) != 0
	if hasImportedCerts {
		return fmt.Errorf("cannot specify alias when env %q imports one or more certificates", d.env.Name)
	}

	if d.app.Domain == "" {
		return fmt.Errorf("cannot specify alias when application is not associated with a domain")
	}

	return validateAliases(d.app, d.env.Name, d.staticSiteMft.HTTP.Alias)
}
