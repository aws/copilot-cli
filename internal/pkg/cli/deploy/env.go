// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
}

type envDeployer struct {
	app *config.Application
	env *config.Environment

	mft manifest.Environment

	appCFN appResourcesGetter

	// Dependencies to upload artifacts.
	uploader customResourcesUploader
	s3       uploader

	// Cached variables.
	appRegionalResources *stack.AppRegionalResources
}

// UploadArtifacts uploads the deployment artifacts for the environment.
func (d *envDeployer) UploadArtifacts() (map[string]string, error) {
	envRegion := d.env.Region

	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}
	if resources.S3Bucket == "" {
		return nil, fmt.Errorf("cannot find the S3 artifact bucket in region %s", envRegion)
	}

	urls, err := d.uploader.UploadEnvironmentCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return d.s3.ZipAndUpload(resources.S3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", resources.S3Bucket, err)
	}
	return urls, nil
}

func (d *envDeployer) getAppRegionalResources() (*stack.AppRegionalResources, error) {
	if d.appRegionalResources != nil {
		return d.appRegionalResources, nil
	}
	resources, err := d.appCFN.GetAppResourcesByRegion(d.app, d.env.Region)
	if err != nil {
		return nil, fmt.Errorf("get app resources in region %s: %w", d.env.Region, err)
	}
	return resources, nil
}
