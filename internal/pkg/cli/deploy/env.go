// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
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

	// Dependencies.
	appCFN appResourcesGetter
	// Dependencies to upload artifacts.
	templateFS template.Reader
	s3         uploader
	// Dependencies to deploy an environment.
	envDeployer environmentDeployer

	// Cached variables.
	appRegionalResources *stack.AppRegionalResources
}

// NewEnvDeployerInput contains information needd to construct an environment deployer.
type NewEnvDeployerInput struct {
	App             *config.Application
	Env             *config.Environment
	SessionProvider *sessions.Provider
}

// NewEnvDeployer constructs an environment deployer.
func NewEnvDeployer(in *NewEnvDeployerInput) (*envDeployer, error) {
	defaultSession, err := in.SessionProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("get default session: %w", err)
	}
	envRegionSession, err := in.SessionProvider.DefaultWithRegion(in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get default session in env region %s: %w", in.Env.Region, err)
	}
	envManagerSession, err := in.SessionProvider.FromRole(in.Env.ManagerRoleARN, in.Env.Region)
	if err != nil {
		return nil, fmt.Errorf("get env session: %w", err)
	}
	return &envDeployer{
		app: in.App,
		env: in.Env,

		appCFN:     deploycfn.New(defaultSession),
		templateFS: template.New(),
		s3:         s3.New(envRegionSession),

		envDeployer: deploycfn.New(envManagerSession),
	}, nil
}

// UploadArtifacts uploads the deployment artifacts for the environment.
func (d *envDeployer) UploadArtifacts() (map[string]string, error) {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}
	return d.uploadCustomResources(resources.S3Bucket)
}

func (d *envDeployer) uploadCustomResources(bucket string) (map[string]string, error) {
	crs, err := customresource.Env(d.templateFS)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for environments: %w", err)
	}
	urls, err := customresource.Upload(func(key string, dat io.Reader) (url string, err error) {
		return d.s3.Upload(bucket, key, dat)
	}, crs)
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", bucket, err)
	}
	return urls, nil
}

// DeployEnvironmentInput contains information used to deploy the environment.
type DeployEnvironmentInput struct {
	RootUserARN         string
	CustomResourcesURLs map[string]string
	Manifest            *manifest.Environment
	ForceNewUpdate      bool
}

// GenerateCloudFormationTemplate returns the environment stack's template and parameter configuration.
func (d *envDeployer) GenerateCloudFormationTemplate(in *DeployEnvironmentInput) (*GenerateCloudFormationTemplateOutput, error) {
	return &GenerateCloudFormationTemplateOutput{}, nil
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
		AdditionalTags:       d.app.Tags,
		CustomResourcesURLs:  in.CustomResourcesURLs,
		ArtifactBucketARN:    s3.FormatARN(partition.ID(), resources.S3Bucket),
		ArtifactBucketKeyARN: resources.KMSKeyARN,
		Mft:                  in.Manifest,
		ForceUpdate:          in.ForceNewUpdate,
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
