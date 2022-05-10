// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
	GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error)
}

type envDeployerInput struct {
	app          *config.Application
	env          *config.Environment
	sessProvider *sessions.Provider
}

func newEnvDeployer(in *envDeployerInput) (*envDeployer, error) {
	defaultSess, err := in.sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	envSess, err := in.sessProvider.FromRole(in.env.ManagerRoleARN, in.env.Region)
	d := &envDeployer{
		appCFN: deploycfn.New(defaultSess),

		uploader: template.New(),
		s3:       s3.New(envSess),
	}
	return d, nil

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
		log.Errorf("Cannot find the S3 artifact bucket in %s region created by app %s. The S3 bucket is necessary for many future operations. For example, when you need addons to your services.", envRegion, d.app.Name)
		return nil, fmt.Errorf("cannot find the S3 artifact bucket in %s region", envRegion)
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
