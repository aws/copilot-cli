// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// Template rendering configuration.
const (
	backlogCalculatorLambdaPath = "custom-resources/backlog-per-task-calculator.js"
)

type workerSvcReadParser interface {
	template.ReadParser
	ParseWorkerService(template.WorkloadOpts) (*template.Content, error)
}

// WorkerService represents the configuration needed to create a CloudFormation stack from a worker service manifest.
type WorkerService struct {
	*ecsWkld
	manifest *manifest.WorkerService

	parser workerSvcReadParser
}

// NewWorkerService creates a new WorkerService stack from a manifest file.
func NewWorkerService(mft *manifest.WorkerService, env, app string, rc RuntimeConfig) (*WorkerService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &WorkerService{
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

// Template returns the CloudFormation template for the worker service.
func (s *WorkerService) Template() (string, error) {
	envControllerLambda, err := s.parser.Read(envControllerPath)
	if err != nil {
		return "", fmt.Errorf("read env controller lambda function source code: %w", err)
	}
	backlogPerTaskLambda, err := s.parser.Read(backlogCalculatorLambdaPath)
	if err != nil {
		return "", fmt.Errorf("read backlog-per-task-calculator lambda function source code: %w", err)
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
	subscribe, err := convertSubscribe(s.manifest.Subscribe, s.rc.AccountID, s.rc.Region, s.app, s.env, s.name)
	if err != nil {
		return "", err
	}
	publishers, err := convertPublish(s.manifest.Publish(), s.rc.AccountID, s.rc.Region, s.app, s.env, s.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for service %s: %w`, s.name, err)
	}
	content, err := s.parser.ParseWorkerService(template.WorkloadOpts{
		Variables:                      s.manifest.WorkerServiceConfig.Variables,
		Secrets:                        s.manifest.WorkerServiceConfig.Secrets,
		NestedStack:                    addonsOutputs,
		AddonsExtraParams:              addonsParams,
		Sidecars:                       sidecars,
		Autoscaling:                    autoscaling,
		CapacityProviders:              capacityProviders,
		DesiredCountOnSpot:             desiredCountOnSpot,
		ExecuteCommand:                 convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:                   manifest.WorkerServiceType,
		HealthCheck:                    convertContainerHealthCheck(s.manifest.WorkerServiceConfig.ImageConfig.HealthCheck),
		LogConfig:                      convertLogging(s.manifest.Logging),
		DockerLabels:                   s.manifest.ImageConfig.Image.DockerLabels,
		EnvControllerLambda:            envControllerLambda.String(),
		BacklogPerTaskCalculatorLambda: backlogPerTaskLambda.String(),
		Storage:                        convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                        convertNetworkConfig(s.manifest.Network),
		EntryPoint:                     entrypoint,
		Command:                        command,
		DependsOn:                      convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:           aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint:       s.rc.ServiceDiscoveryEndpoint,
		Subscribe:                      subscribe,
		Publish:                        publishers,
		Platform:                       convertPlatform(s.manifest.Platform),
	})
	if err != nil {
		return "", fmt.Errorf("parse worker service template: %w", err)
	}
	overridenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overridenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *WorkerService) Parameters() ([]*cloudformation.Parameter, error) {
	svcParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	return svcParams, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *WorkerService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
