// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// WorkerService represents the configuration needed to create a CloudFormation stack from a worker service manifest.
type WorkerService struct {
	*ecsWkld
	manifest *manifest.WorkerService

	parser workerSvcReadParser
}

// WorkerServiceConfig contains data required to initialize a scheduled job stack.
type WorkerServiceConfig struct {
	App                *config.Application
	Env                string
	Manifest           *manifest.WorkerService
	ArtifactBucketName string
	ArtifactKey        string
	RawManifest        string
	RuntimeConfig      RuntimeConfig
	Addons             NestedStackConfigurer
}

// NewWorkerService creates a new WorkerService stack from a manifest file.
func NewWorkerService(cfg WorkerServiceConfig) (*WorkerService, error) {
	crs, err := customresource.Worker(fs)
	if err != nil {
		return nil, fmt.Errorf("worker service custom resources: %w", err)
	}
	cfg.RuntimeConfig.loadCustomResourceURLs(cfg.ArtifactBucketName, uploadableCRs(crs).convert())

	return &WorkerService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:               aws.StringValue(cfg.Manifest.Name),
				env:                cfg.Env,
				app:                cfg.App.Name,
				permBound:          cfg.App.PermissionsBoundary,
				artifactBucketName: cfg.ArtifactBucketName,
				artifactKey:        cfg.ArtifactKey,
				rc:                 cfg.RuntimeConfig,
				image:              cfg.Manifest.ImageConfig.Image,
				rawManifest:        cfg.RawManifest,
				parser:             fs,
				addons:             cfg.Addons,
			},
			logging:             cfg.Manifest.Logging,
			sidecars:            cfg.Manifest.Sidecars,
			tc:                  cfg.Manifest.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest: cfg.Manifest,
		parser:   fs,
	}, nil
}

// Template returns the CloudFormation template for the worker service.
func (s *WorkerService) Template() (string, error) {
	crs, err := convertCustomResources(s.rc.CustomResourcesURL)
	if err != nil {
		return "", err
	}
	addonsParams, err := s.addonsParameters()
	if err != nil {
		return "", err
	}
	addonsOutputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	exposedPorts, err := s.manifest.ExposedPorts()
	if err != nil {
		return "", fmt.Errorf("parse exposed ports in service manifest %s: %w", s.name, err)
	}
	sidecars, err := convertSidecars(s.manifest.Sidecars, exposedPorts.PortsForContainer, s.rc)
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for service %s: %w", s.name, err)
	}
	advancedCount, err := convertAdvancedCount(s.manifest.Count.AdvancedCount)
	if err != nil {
		return "", fmt.Errorf("convert the advanced count configuration for service %s: %w", s.name, err)
	}

	var autoscaling *template.AutoscalingOpts
	var desiredCountOnSpot *int
	var capacityProviders []*template.CapacityProviderStrategy

	if advancedCount != nil {
		autoscaling = advancedCount.Autoscaling
		desiredCountOnSpot = advancedCount.Spot
		capacityProviders = advancedCount.Cps
	}

	entrypoint, err := convertEntryPoint(s.manifest.EntryPoint)
	if err != nil {
		return "", err
	}
	command, err := convertCommand(s.manifest.Command)
	if err != nil {
		return "", err
	}
	subscribe, err := convertSubscribe(s.manifest)
	if err != nil {
		return "", err
	}
	publishers, err := convertPublish(s.manifest.Publish(), s.rc.AccountID, s.rc.Region, s.app, s.env, s.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for service %s: %w`, s.name, err)
	}
	scOpts := template.ServiceConnectOpts{
		Server: convertServiceConnectServer(s.manifest.Network.Connect, nil),
		Client: s.manifest.Network.Connect.Enabled(),
	}
	content, err := s.parser.ParseWorkerService(template.WorkloadOpts{
		AppName:                  s.app,
		EnvName:                  s.env,
		WorkloadName:             s.name,
		SerializedManifest:       string(s.rawManifest),
		EnvVersion:               s.rc.EnvVersion,
		Version:                  s.rc.Version,
		Variables:                convertEnvVars(s.manifest.WorkerServiceConfig.Variables),
		Secrets:                  convertSecrets(s.manifest.WorkerServiceConfig.Secrets),
		NestedStack:              addonsOutputs,
		AddonsExtraParams:        addonsParams,
		Sidecars:                 sidecars,
		Autoscaling:              autoscaling,
		CapacityProviders:        capacityProviders,
		DesiredCountOnSpot:       desiredCountOnSpot,
		ExecuteCommand:           convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:             manifestinfo.WorkerServiceType,
		HealthCheck:              convertContainerHealthCheck(s.manifest.WorkerServiceConfig.ImageConfig.HealthCheck),
		LogConfig:                convertLogging(s.manifest.Logging),
		DockerLabels:             s.manifest.ImageConfig.Image.DockerLabels,
		CustomResources:          crs,
		Storage:                  convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                  convertNetworkConfig(s.manifest.Network),
		DeploymentConfiguration:  convertWorkerDeploymentConfig(s.manifest.WorkerServiceConfig.DeployConfig),
		EntryPoint:               entrypoint,
		ServiceConnectOpts:       scOpts,
		Command:                  command,
		DependsOn:                convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:     aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,
		Subscribe:                subscribe,
		Publish:                  publishers,
		Platform:                 convertPlatform(s.manifest.Platform),
		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},
		PermissionsBoundary: s.permBound,
	})
	if err != nil {
		return "", fmt.Errorf("parse worker service template: %w", err)
	}
	overriddenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overriddenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *WorkerService) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	return wkldParams, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (s *WorkerService) SerializedParameters() (string, error) {
	return serializeTemplateConfig(s.wkld.parser, s)
}
