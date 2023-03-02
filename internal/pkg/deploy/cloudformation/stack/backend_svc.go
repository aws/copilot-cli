// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// BackendService represents the configuration needed to create a CloudFormation stack from a backend service manifest.
type BackendService struct {
	*ecsWkld
	manifest     *manifest.BackendService
	httpsEnabled bool
	albEnabled   bool

	parser backendSvcReadParser
}

// BackendServiceConfig contains data required to initialize a backend service stack.
type BackendServiceConfig struct {
	App                *config.Application
	EnvManifest        *manifest.Environment
	Manifest           *manifest.BackendService
	ArtifactBucketName string
	RawManifest        []byte // Content of the manifest file without any transformations.
	RuntimeConfig      RuntimeConfig
	Addons             NestedStackConfigurer
}

// NewBackendService creates a new BackendService stack from a manifest file.
func NewBackendService(conf BackendServiceConfig) (*BackendService, error) {
	crs, err := customresource.Backend(fs)
	if err != nil {
		return nil, fmt.Errorf("backend service custom resources: %w", err)
	}
	conf.RuntimeConfig.loadCustomResourceURLs(conf.ArtifactBucketName, uploadableCRs(crs).convert())

	b := &BackendService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:               aws.StringValue(conf.Manifest.Name),
				env:                aws.StringValue(conf.EnvManifest.Name),
				app:                conf.App.Name,
				permBound:          conf.App.PermissionsBoundary,
				artifactBucketName: conf.ArtifactBucketName,
				rc:                 conf.RuntimeConfig,
				image:              conf.Manifest.ImageConfig.Image,
				rawManifest:        conf.RawManifest,
				parser:             fs,
				addons:             conf.Addons,
			},
			logRetention:        conf.Manifest.Logging.Retention,
			tc:                  conf.Manifest.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest:   conf.Manifest,
		parser:     fs,
		albEnabled: !conf.Manifest.RoutingRule.IsEmpty(),
	}

	if len(conf.EnvManifest.HTTPConfig.Private.Certificates) != 0 {
		b.httpsEnabled = b.albEnabled
	}

	return b, nil
}

// Template returns the CloudFormation template for the backend service.
func (s *BackendService) Template() (string, error) {
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
	sidecars, err := convertSidecars(s.manifest.Sidecars, exposedPorts.PortsForContainer)
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
	var deregistrationDelay *int64 = aws.Int64(60)
	if s.manifest.RoutingRule.DeregistrationDelay != nil {
		deregistrationDelay = aws.Int64(int64(s.manifest.RoutingRule.DeregistrationDelay.Seconds()))
	}
	var scConfig *template.ServiceConnect
	if s.manifest.Network.Connect.Enabled() {
		scConfig = convertServiceConnect(s.manifest.Network.Connect)
	}
	targetContainer, targetContainerPort, err := s.manifest.HTTPLoadBalancerTarget()
	if err != nil {
		return "", err
	}

	albConfig, err := s.convertApplicationLoadBalancer()
	if err != nil {
		return "", err
	}

	content, err := s.parser.ParseBackendService(template.WorkloadOpts{
		// Workload parameters.
		AppName:            s.app,
		EnvName:            s.env,
		EnvVersion:         s.rc.EnvVersion,
		SerializedManifest: string(s.rawManifest),
		WorkloadType:       manifestinfo.BackendServiceType,
		WorkloadName:       s.name,

		// Configuration for the main container.
		EntryPoint:   entrypoint,
		Command:      command,
		HealthCheck:  convertContainerHealthCheck(s.manifest.BackendServiceConfig.ImageConfig.HealthCheck),
		PortMappings: convertPortMappings(exposedPorts.PortsForContainer[s.name]),
		Secrets:      convertSecrets(s.manifest.BackendServiceConfig.Secrets),
		Variables:    convertEnvVars(s.manifest.BackendServiceConfig.Variables),

		// Additional options that are common between **all** workload templates.
		AddonsExtraParams:       addonsParams,
		Autoscaling:             autoscaling,
		CapacityProviders:       capacityProviders,
		CredentialsParameter:    aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		DeploymentConfiguration: convertDeploymentConfig(s.manifest.DeployConfig),
		DesiredCountOnSpot:      desiredCountOnSpot,
		DependsOn:               convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		DockerLabels:            s.manifest.ImageConfig.Image.DockerLabels,
		ExecuteCommand:          convertExecuteCommand(&s.manifest.ExecuteCommand),
		LogConfig:               convertLogging(s.manifest.Logging),
		NestedStack:             addonsOutputs,
		Network:                 convertNetworkConfig(s.manifest.Network),
		Publish:                 publishers,
		PermissionsBoundary:     s.permBound,
		Platform:                convertPlatform(s.manifest.Platform),
		Storage:                 convertStorageOpts(s.manifest.Name, s.manifest.Storage),

		// ALB configs.
		ALBEnabled:          s.albEnabled,
		DeregistrationDelay: deregistrationDelay,
		HTTPTargetContainer: template.HTTPTargetContainer{
			Port: targetContainerPort,
			Name: targetContainer,
		},
		HTTPHealthCheck: convertHTTPHealthCheck(&s.manifest.RoutingRule.HealthCheck),
		ALB:             albConfig,

		// Custom Resource Config.
		CustomResources: crs,

		// Sidecar config.
		Sidecars: sidecars,

		// service connect and service discovery options.
		ServiceConnect:           scConfig,
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,

		// Additional options for request driven web service templates.
		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("parse backend service template: %w", err)
	}
	overriddenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overriddenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *BackendService) Parameters() ([]*cloudformation.Parameter, error) {
	params, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	targetContainer, targetPort, err := s.manifest.HTTPLoadBalancerTarget()
	if err != nil {
		return nil, err
	}
	params = append(params, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String(s.manifest.MainContainerPort()),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(s.rc.EnvFileARN),
		},
		{
			ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
			ParameterValue: aws.String(targetContainer),
		},
		{
			ParameterKey:   aws.String(WorkloadTargetPortParamKey),
			ParameterValue: aws.String(targetPort),
		},
	}...)

	if !s.manifest.RoutingRule.IsEmpty() {
		params = append(params, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(WorkloadRulePathParamKey),
				ParameterValue: s.manifest.RoutingRule.Path,
			},
			{
				ParameterKey:   aws.String(WorkloadStickinessParamKey),
				ParameterValue: aws.String(strconv.FormatBool(aws.BoolValue(s.manifest.RoutingRule.Stickiness))),
			},
			{
				ParameterKey:   aws.String(WorkloadHTTPSParamKey),
				ParameterValue: aws.String(strconv.FormatBool(s.httpsEnabled)),
			},
		}...)
	}

	return params, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (s *BackendService) SerializedParameters() (string, error) {
	return serializeTemplateConfig(s.wkld.parser, s)
}
