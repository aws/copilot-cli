// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// Parameter logical IDs for a backend service.
const (
	BackendServiceContainerPortParamKey = "ContainerPort"
)

const (
	// NoExposedContainerPort indicates no port should be exposed for the service container.
	NoExposedContainerPort = "-1"
)

type backendSvcReadParser interface {
	template.ReadParser
	ParseBackendService(template.WorkloadOpts) (*template.Content, error)
}

// BackendService represents the configuration needed to create a CloudFormation stack from a backend service manifest.
type BackendService struct {
	*ecsWkld
	manifest *manifest.BackendService

	parser backendSvcReadParser
}

// NewBackendService creates a new BackendService stack from a manifest file.
func NewBackendService(mft *manifest.BackendService, env, app string, rc RuntimeConfig) (*BackendService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &BackendService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:   aws.StringValue(mft.Name),
				env:    env,
				app:    app,
				rc:     rc,
				image:  mft.ImageConfig.Image,
				parser: parser,
				addons: addons,
			},
			logRetention:        mft.Logging.Retention,
			tc:                  mft.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest: mft,

		parser: parser,
	}, nil
}

// Template returns the CloudFormation template for the backend service.
func (s *BackendService) Template() (string, error) {
	desiredCountLambda, err := s.parser.Read(desiredCountGeneratorPath)
	if err != nil {
		return "", fmt.Errorf("read desired count lambda: %w", err)
	}
	envControllerLambda, err := s.parser.Read(envControllerPath)
	if err != nil {
		return "", fmt.Errorf("read env controller lambda: %w", err)
	}
	addonsParams, err := s.addonsParameters()
	if err != nil {
		return "", err
	}
	addonsOutputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	sidecars, err := convertSidecar(s.manifest.Sidecars)
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for service %s: %w", s.name, err)
	}
	publishers, err := convertPublish(s.manifest.Publish(), s.rc.AccountID, s.rc.Region, s.app, s.env, s.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for service %s: %w`, s.name, err)
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
	content, err := s.parser.ParseBackendService(template.WorkloadOpts{
		Variables:                s.manifest.BackendServiceConfig.Variables,
		Secrets:                  convertSecrets(s.manifest.BackendServiceConfig.Secrets),
		NestedStack:              addonsOutputs,
		AddonsExtraParams:        addonsParams,
		Sidecars:                 sidecars,
		Autoscaling:              autoscaling,
		CapacityProviders:        capacityProviders,
		DesiredCountOnSpot:       desiredCountOnSpot,
		ExecuteCommand:           convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:             manifest.BackendServiceType,
		HealthCheck:              convertContainerHealthCheck(s.manifest.BackendServiceConfig.ImageConfig.HealthCheck),
		LogConfig:                convertLogging(s.manifest.Logging),
		DockerLabels:             s.manifest.ImageConfig.Image.DockerLabels,
		DesiredCountLambda:       desiredCountLambda.String(),
		EnvControllerLambda:      envControllerLambda.String(),
		Storage:                  convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                  convertNetworkConfig(s.manifest.Network),
		EntryPoint:               entrypoint,
		Command:                  command,
		DependsOn:                convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:     aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,
		Publish:                  publishers,
		Platform:                 convertPlatform(s.manifest.Platform),
	})
	if err != nil {
		return "", fmt.Errorf("parse backend service template: %w", err)
	}
	overridenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overridenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *BackendService) Parameters() ([]*cloudformation.Parameter, error) {
	svcParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	containerPort := NoExposedContainerPort
	if s.manifest.BackendServiceConfig.ImageConfig.Port != nil {
		containerPort = strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.BackendServiceConfig.ImageConfig.Port)), 10)
	}
	return append(svcParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(BackendServiceContainerPortParamKey),
			ParameterValue: aws.String(containerPort),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(s.rc.EnvFileARN),
		},
	}...), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *BackendService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
