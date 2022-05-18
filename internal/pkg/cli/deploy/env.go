// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"os"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
)

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
}

type environmentDeployer interface {
	UpdateAndRenderEnvironment(out termprogress.FileWriter, env *deploy.CreateEnvironmentInput, opts ...cloudformation.StackOption) error
}

type envDeployer struct {
	app *config.Application
	env *config.Environment

	appCFN appResourcesGetter

	// Dependencies to upload artifacts.
	uploader customResourcesUploader
	s3       uploader

	// Dependencies to deploy an environment.
	envDeployer environmentDeployer

	// Cached variables.
	appRegionalResources *stack.AppRegionalResources
}

// UploadArtifacts uploads the deployment artifacts for the environment.
func (d *envDeployer) UploadArtifacts() (map[string]string, error) {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}

	urls, err := d.uploader.UploadEnvironmentCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return d.s3.ZipAndUpload(resources.S3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", resources.S3Bucket, err)
	}
	return urls, nil
}

// DeployEnvironmentInput contains information used to deploy the environment.
type DeployEnvironmentInput struct {
	RootUserARN         string
	IsProduction        bool
	CustomResourcesURLs map[string]string
	Manifest            *manifest.Environment
}

// DeployEnvironment deploys an environment using CloudFormation.
func (d *envDeployer) DeployEnvironment(in *DeployEnvironmentInput) error {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return err
	}
	partition, err := partitions.Region(d.env.Region).Partition()
	if err != nil {
		return err
	}
	deployEnvInput := &deploy.CreateEnvironmentInput{
		Name: d.env.Name,
		App: deploy.AppInformation{
			Name:                d.app.Name,
			Domain:              d.app.Domain,
			AccountPrincipalARN: in.RootUserARN,
		},
		Prod:                 in.IsProduction,
		AdditionalTags:       d.app.Tags,
		CustomResourcesURLs:  in.CustomResourcesURLs,
		ArtifactBucketARN:    s3.FormatARN(partition.ID(), resources.S3Bucket),
		ArtifactBucketKeyARN: resources.KMSKeyARN,
		Mft:                  in.Manifest,
		Version:              deploy.LatestEnvTemplateVersion,
	}
	return d.envDeployer.UpdateAndRenderEnvironment(os.Stderr, deployEnvInput, cloudformation.WithRoleARN(d.env.ExecutionRoleARN))
}

func (d *envDeployer) getAppRegionalResources() (*stack.AppRegionalResources, error) {
	if d.appRegionalResources != nil {
		return d.appRegionalResources, nil
	}
	resources, err := d.appCFN.GetAppResourcesByRegion(d.app, d.env.Region)
	if err != nil {
		return nil, fmt.Errorf("get app resources in region %s: %w", d.env.Region, err)
	}
	if resources.S3Bucket == "" {
		return nil, fmt.Errorf("cannot find the S3 artifact bucket in region %s", d.env.Region)
	}
	return resources, nil
}
