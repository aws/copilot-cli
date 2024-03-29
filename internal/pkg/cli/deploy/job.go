// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"

	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type jobDeployer struct {
	*workloadDeployer
	jobMft *manifest.ScheduledJob

	// Overriden in tests.
	newStack func() cloudformation.StackConfiguration
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (jobDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
}

// NewJobDeployer is the constructor for jobDeployer.
func NewJobDeployer(in *WorkloadDeployerInput) (*jobDeployer, error) {
	in.customResources = scheduledJobCustomResources
	wkldDeployer, err := newWorkloadDeployer(in)
	if err != nil {
		return nil, err
	}
	jobMft, ok := in.Mft.(*manifest.ScheduledJob)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.ScheduledJobType)
	}
	return &jobDeployer{
		workloadDeployer: wkldDeployer,
		jobMft:           jobMft,
	}, nil
}

func scheduledJobCustomResources(fs template.Reader) ([]*customresource.CustomResource, error) {
	crs, err := customresource.ScheduledJob(fs)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for a %q: %w", manifestinfo.ScheduledJobType, err)
	}
	return crs, nil
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *jobDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.buildAndPushContainerImages, d.uploadArtifactsToS3, d.uploadCustomResources)
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *jobDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a job using CloudFormation.
func (d *jobDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	opts := []awscloudformation.StackOption{
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN),
	}
	if in.DisableRollback {
		opts = append(opts, awscloudformation.WithDisableRollback())
	}
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deployer.DeployService(stackConfigOutput.conf, d.resources.S3Bucket, in.Detach, opts...); err != nil {
		return nil, fmt.Errorf("deploy job: %w", err)
	}
	return noopActionRecommender{}, nil
}

type jobStackConfigurationOutput struct {
	conf cloudformation.StackConfiguration
}

func (d *jobDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*jobStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}

	var conf cloudformation.StackConfiguration
	switch {
	case d.newStack != nil:
		conf = d.newStack()
	default:
		conf, err = stack.NewScheduledJob(stack.ScheduledJobConfig{
			App:                d.app,
			Env:                d.env.Name,
			Manifest:           d.jobMft,
			RawManifest:        d.rawMft,
			ArtifactBucketName: d.resources.S3Bucket,
			ArtifactKey:        d.resources.KMSKeyARN,
			RuntimeConfig:      *rc,
			Addons:             d.addons,
		})
		if err != nil {
			return nil, fmt.Errorf("create stack configuration: %w", err)
		}
	}

	return &jobStackConfigurationOutput{
		conf: cloudformation.WrapWithTemplateOverrider(conf, d.overrider),
	}, nil
}
