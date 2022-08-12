// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"io"
	"os"

	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
}

type environmentDeployer interface {
	UpdateAndRenderEnvironment(conf deploycfn.StackConfiguration, bucketARN string, opts ...cloudformation.StackOption) error
	EnvironmentParameters(app, env string) ([]*awscfn.Parameter, error)
	ForceUpdateOutputID(app, env string) (string, error)
}

type prefixListGetter interface {
	CloudFrontManagedPrefixListID() (string, error)
}

type envDeployer struct {
	app *config.Application
	env *config.Environment

	// Dependencies to upload artifacts.
	templateFS       template.Reader
	s3               uploader
	prefixListGetter prefixListGetter
	// Dependencies to deploy an environment.
	appCFN             appResourcesGetter
	envDeployer        environmentDeployer
	newStackSerializer func(input *deploy.CreateEnvironmentInput, forceUpdateID string, prevParams []*awscfn.Parameter) stackSerializer

	// Cached variables.
	appRegionalResources *stack.AppRegionalResources
}

// NewEnvDeployerInput contains information needed to construct an environment deployer.
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

		templateFS:       template.New(),
		s3:               s3.New(envRegionSession),
		prefixListGetter: ec2.New(envRegionSession),

		appCFN:      deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		envDeployer: deploycfn.New(envManagerSession, deploycfn.WithProgressTracker(os.Stderr)),
		newStackSerializer: func(in *deploy.CreateEnvironmentInput, lastForceUpdateID string, oldParams []*awscfn.Parameter) stackSerializer {
			return stack.NewEnvConfigFromExistingStack(in, lastForceUpdateID, oldParams)
		},
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

func (d *envDeployer) cidrPrefixLists(in *DeployEnvironmentInput) ([]string, error) {
	var cidrPrefixListIDs []string

	// Check if ingress is allowed from cloudfront
	if in.Manifest == nil || !in.Manifest.IsIngressRestrictedToCDN() {
		return nil, nil
	}
	cfManagedPrefixListID, err := d.cfManagedPrefixListID()
	if err != nil {
		return nil, err
	}
	cidrPrefixListIDs = append(cidrPrefixListIDs, cfManagedPrefixListID)

	return cidrPrefixListIDs, nil
}

func (d *envDeployer) cfManagedPrefixListID() (string, error) {
	id, err := d.prefixListGetter.CloudFrontManagedPrefixListID()
	if err != nil {
		return "", fmt.Errorf("retrieve CloudFront managed prefix list id: %w", err)
	}

	return id, nil
}

// DeployEnvironmentInput contains information used to deploy the environment.
type DeployEnvironmentInput struct {
	RootUserARN         string
	CustomResourcesURLs map[string]string
	Manifest            *manifest.Environment
	ForceNewUpdate      bool
	RawManifest         []byte
}

// GenerateCloudFormationTemplate returns the environment stack's template and parameter configuration.
func (d *envDeployer) GenerateCloudFormationTemplate(in *DeployEnvironmentInput) (*GenerateCloudFormationTemplateOutput, error) {
	stackInput, err := d.buildStackInput(in)
	if err != nil {
		return nil, err
	}
	oldParams, err := d.envDeployer.EnvironmentParameters(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("describe environment stack parameters: %w", err)
	}
	lastForceUpdateID, err := d.envDeployer.ForceUpdateOutputID(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("retrieve environment stack force update ID: %w", err)
	}
	stack := d.newStackSerializer(stackInput, lastForceUpdateID, oldParams)
	tpl, err := stack.Template()
	if err != nil {
		return nil, fmt.Errorf("generate stack template: %w", err)
	}
	params, err := stack.SerializedParameters()
	if err != nil {
		return nil, fmt.Errorf("generate stack template parameters: %w", err)
	}
	return &GenerateCloudFormationTemplateOutput{
		Template:   tpl,
		Parameters: params,
	}, nil
}

// DeployEnvironment deploys an environment using CloudFormation.
func (d *envDeployer) DeployEnvironment(in *DeployEnvironmentInput) error {
	stackInput, err := d.buildStackInput(in)
	if err != nil {
		return err
	}
	oldParams, err := d.envDeployer.EnvironmentParameters(d.app.Name, d.env.Name)
	if err != nil {
		return fmt.Errorf("describe environment stack parameters: %w", err)
	}
	lastForceUpdateID, err := d.envDeployer.ForceUpdateOutputID(d.app.Name, d.env.Name)
	if err != nil {
		return fmt.Errorf("retrieve environment stack force update ID: %w", err)
	}
	conf := stack.NewEnvConfigFromExistingStack(stackInput, lastForceUpdateID, oldParams)
	return d.envDeployer.UpdateAndRenderEnvironment(conf, stackInput.ArtifactBucketARN, cloudformation.WithRoleARN(d.env.ExecutionRoleARN))
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

func (d *envDeployer) buildStackInput(in *DeployEnvironmentInput) (*deploy.CreateEnvironmentInput, error) {
	resources, err := d.getAppRegionalResources()
	if err != nil {
		return nil, err
	}
	partition, err := partitions.Region(d.env.Region).Partition()
	if err != nil {
		return nil, err
	}
	cidrPrefixListIDs, err := d.cidrPrefixLists(in)
	if err != nil {
		return nil, err
	}
	return &deploy.CreateEnvironmentInput{
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
		CIDRPrefixListIDs:    cidrPrefixListIDs,
		Mft:                  in.Manifest,
		ForceUpdate:          in.ForceNewUpdate,
		RawMft:               in.RawManifest,
		Version:              deploy.LatestEnvTemplateVersion,
	}, nil
}
