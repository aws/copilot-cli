// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"io"

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
	"github.com/spf13/afero"
)

const artifactBucketAssetsDir = "local-assets"

type fileUploader interface {
	UploadFiles(files []manifest.FileUpload) (string, error)
}

type staticSiteDeployer struct {
	*svcDeployer
	appVersionGetter versionGetter
	staticSiteMft    *manifest.StaticSite
	fs               afero.Fs
	uploader         fileUploader
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

// UploadArtifacts uploads static assets to the app stackset bucket.
func (d *staticSiteDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.uploadStaticFiles, d.uploadArtifactsToS3, d.uploadCustomResources)
}

func (d *staticSiteDeployer) uploadStaticFiles(out *UploadArtifactsOutput) error {
	path, err := d.uploader.UploadFiles(d.staticSiteMft.FileUploads)
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
	if err := validateMinAppVersion(d.app.Name, d.name, d.appVersionGetter, deploy.StaticSiteMinAppTemplateVersion); err != nil {
		return nil, fmt.Errorf("static sites not supported: %w", err)
	}
	if err := d.validateSources(); err != nil {
		return nil, err
	}
	conf, err := stack.NewStaticSite(&stack.StaticSiteConfig{
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

func (d *staticSiteDeployer) validateSources() error {
	for _, upload := range d.staticSiteMft.FileUploads {
		_, err := d.fs.Stat(upload.Source)
		if err != nil {
			return fmt.Errorf("source '%s' must be a valid path: %w", upload.Source, err)
		}
	}
	return nil
}
