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
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
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
	manifest     *manifest.BackendService
	httpsEnabled bool
	albEnabled   bool

	parser backendSvcReadParser
}

// BackendServiceConfig contains data required to initialize a backend service stack.
type BackendServiceConfig struct {
	App           *config.Application
	EnvManifest   *manifest.Environment
	Manifest      *manifest.BackendService
	RawManifest   []byte // Content of the manifest file without any transformations.
	RuntimeConfig RuntimeConfig
	Addons        addons
}

// NewBackendService creates a new BackendService stack from a manifest file.
func NewBackendService(conf BackendServiceConfig) (*BackendService, error) {
	parser := template.New()
	b := &BackendService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:        aws.StringValue(conf.Manifest.Name),
				env:         aws.StringValue(conf.EnvManifest.Name),
				app:         conf.App.Name,
				rc:          conf.RuntimeConfig,
				image:       conf.Manifest.ImageConfig.Image,
				rawManifest: conf.RawManifest,
				parser:      parser,
				addons:      conf.Addons,
			},
			logRetention:        conf.Manifest.Logging.Retention,
			tc:                  conf.Manifest.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest:   conf.Manifest,
		parser:     parser,
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
	var aliases []string
	if s.httpsEnabled {
		if aliases, err = convertAlias(s.manifest.RoutingRule.Alias); err != nil {
			return "", err
		}
	}
	hostedZoneAliases, err := convertHostedZone(s.manifest.RoutingRule)
	if err != nil {
		return "", err
	}
	var deregistrationDelay *int64 = aws.Int64(60)
	if s.manifest.RoutingRule.DeregistrationDelay != nil {
		deregistrationDelay = aws.Int64(int64(s.manifest.RoutingRule.DeregistrationDelay.Seconds()))
	}
	var allowedSourceIPs []string
	for _, ipNet := range s.manifest.RoutingRule.AllowedSourceIps {
		allowedSourceIPs = append(allowedSourceIPs, string(ipNet))
	}

	content, err := s.parser.ParseBackendService(template.WorkloadOpts{
		AppName:            s.app,
		EnvName:            s.env,
		WorkloadName:       s.name,
		SerializedManifest: string(s.rawManifest),

		Variables:                s.manifest.BackendServiceConfig.Variables,
		Secrets:                  convertSecrets(s.manifest.BackendServiceConfig.Secrets),
		Aliases:                  aliases,
		HTTPSListener:            s.httpsEnabled,
		NestedStack:              addonsOutputs,
		AddonsExtraParams:        addonsParams,
		Sidecars:                 sidecars,
		Autoscaling:              autoscaling,
		CapacityProviders:        capacityProviders,
		DesiredCountOnSpot:       desiredCountOnSpot,
		ExecuteCommand:           convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:             manifest.BackendServiceType,
		HealthCheck:              convertContainerHealthCheck(s.manifest.BackendServiceConfig.ImageConfig.HealthCheck),
		HTTPHealthCheck:          convertHTTPHealthCheck(&s.manifest.RoutingRule.HealthCheck),
		DeregistrationDelay:      deregistrationDelay,
		AllowedSourceIps:         allowedSourceIPs,
		CustomResources:          crs,
		LogConfig:                convertLogging(s.manifest.Logging),
		DockerLabels:             s.manifest.ImageConfig.Image.DockerLabels,
		Storage:                  convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                  convertNetworkConfig(s.manifest.Network),
		DeploymentConfiguration:  convertDeploymentConfig(s.manifest.DeployConfig),
		EntryPoint:               entrypoint,
		Command:                  command,
		DependsOn:                convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:     aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,
		Publish:                  publishers,
		Platform:                 convertPlatform(s.manifest.Platform),
		HTTPVersion:              convertHTTPVersion(s.manifest.RoutingRule.ProtocolVersion),
		ALBEnabled:               s.albEnabled,
		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},
		HostedZoneAliases: hostedZoneAliases,
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

func (s *BackendService) httpLoadBalancerTarget() (targetContainer *string, targetPort *string) {
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(s.name)
	targetPort = aws.String(s.containerPort())

	rrTarget := s.manifest.RoutingRule.GetTargetContainer()
	if rrTarget != nil && *rrTarget != *targetContainer {
		targetContainer = rrTarget
		targetPort = s.manifest.Sidecars[aws.StringValue(targetContainer)].Port
	}

	return
}

func (s *BackendService) containerPort() string {
	port := NoExposedContainerPort
	if s.manifest.BackendServiceConfig.ImageConfig.Port != nil {
		port = strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.BackendServiceConfig.ImageConfig.Port)), 10)
	}

	return port
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *BackendService) Parameters() ([]*cloudformation.Parameter, error) {
	params, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}

	targetContainer, targetPort := s.httpLoadBalancerTarget()
	params = append(params, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String(s.containerPort()),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(s.rc.EnvFileARN),
		},
	}...)

	if !s.manifest.RoutingRule.IsEmpty() {
		params = append(params, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
				ParameterValue: targetContainer,
			},
			{
				ParameterKey:   aws.String(WorkloadTargetPortParamKey),
				ParameterValue: targetPort,
			},
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
