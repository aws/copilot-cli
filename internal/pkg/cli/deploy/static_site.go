// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/asset"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/spf13/afero"
)

type staticSiteDeployer struct {
	*svcDeployer
	staticSiteMft *manifest.StaticSite
	bucketName    string
	fs            afero.Fs
	uploadFn      func(fs afero.Fs, source, destination string, opts *asset.UploadOpts) ([]string, error)
}

// NewStaticSiteDeployer is the constructor for staticSiteDeployer.
func NewStaticSiteDeployer(in *WorkloadDeployerInput) (*staticSiteDeployer, error) {
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	mft, ok := in.Mft.(*manifest.StaticSite)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.StaticSiteType)
	}
	return &staticSiteDeployer{
		svcDeployer:   svcDeployer,
		staticSiteMft: mft,
		fs:            afero.NewOsFs(),
		bucketName:    svcDeployer.resources.S3Bucket,
		uploadFn:      asset.Upload,
	}, nil
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (*staticSiteDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(s3.EndpointsID, region)
}

// DeployWorkload deploys a static site service using CloudFormation.
func (d *staticSiteDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	return nil, nil
}

// UploadArtifacts uploads static assets to the app stackset bucket.
func (d *staticSiteDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	for _, f := range d.staticSiteMft.FileUploads {
		fullSource := filepath.Join(f.Context, f.Source)
		_, err := d.uploadFn(d.fs, fullSource, f.Destination, &asset.UploadOpts{
			Reincludes: f.Reinclude.ToStringSlice(),
			Excludes:   f.Exclude.ToStringSlice(),
			Recursive:  f.Recursive,
			UploadFn: func(key string, contents io.Reader) (string, error) {
				return d.s3Client.Upload(d.bucketName, key, contents)
			},
		})
		if err != nil {
			return nil, err
		}
	}
	return &UploadArtifactsOutput{}, nil
}
