// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// Parameter logical IDs for a load balanced web service.
const (
	LBWebServiceDNSDelegatedParamKey = "DNSDelegated"
	LBWebServiceNLBAliasesParamKey   = "NLBAliases"
	LBWebServiceNLBPortParamKey      = "NLBPort"
)

// LoadBalancedWebService represents the configuration needed to create a CloudFormation stack from a load balanced web service manifest.
type LoadBalancedWebService struct {
	*ecsWkld
	manifest             *manifest.LoadBalancedWebService
	httpsEnabled         bool
	dnsDelegationEnabled bool
	importedALB          *elbv2.LoadBalancer
	appInfo              deploy.AppInformation

	parser loadBalancedWebSvcReadParser
}

// LoadBalancedWebServiceOption is used to configuring an optional field for LoadBalancedWebService.
type LoadBalancedWebServiceOption func(s *LoadBalancedWebService)

// WithImportedALB specifies an imported load balancer.
func WithImportedALB(alb *elbv2.LoadBalancer) func(s *LoadBalancedWebService) {
	return func(s *LoadBalancedWebService) {
		s.importedALB = alb
	}
}

// LoadBalancedWebServiceConfig contains fields to configure LoadBalancedWebService.
type LoadBalancedWebServiceConfig struct {
	App                *config.Application
	EnvManifest        *manifest.Environment
	Manifest           *manifest.LoadBalancedWebService
	RawManifest        string
	RuntimeConfig      RuntimeConfig
	RootUserARN        string
	ArtifactBucketName string
	ArtifactKey        string
	Addons             NestedStackConfigurer
}

// NewLoadBalancedWebService creates a new CFN stack with an ECS service from a manifest file, given the options.
func NewLoadBalancedWebService(conf LoadBalancedWebServiceConfig,
	opts ...LoadBalancedWebServiceOption) (*LoadBalancedWebService, error) {
	crs, err := customresource.LBWS(fs)
	if err != nil {
		return nil, fmt.Errorf("load balanced web service custom resources: %w", err)
	}
	conf.RuntimeConfig.loadCustomResourceURLs(conf.ArtifactBucketName, uploadableCRs(crs).convert())

	var dnsDelegationEnabled, httpsEnabled bool
	var appInfo deploy.AppInformation
	if conf.App.Domain != "" {
		dnsDelegationEnabled = true
		appInfo = deploy.AppInformation{
			Name:                conf.App.Name,
			Domain:              conf.App.Domain,
			AccountPrincipalARN: conf.RootUserARN,
		}
		httpsEnabled = true
	}
	certImported := len(conf.EnvManifest.HTTPConfig.Public.Certificates) != 0
	if certImported {
		httpsEnabled = true
		dnsDelegationEnabled = false
	}
	if conf.Manifest.HTTPOrBool.Disabled() {
		httpsEnabled = false
	}
	s := &LoadBalancedWebService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:               aws.StringValue(conf.Manifest.Name),
				env:                aws.StringValue(conf.EnvManifest.Name),
				app:                conf.App.Name,
				permBound:          conf.App.PermissionsBoundary,
				artifactBucketName: conf.ArtifactBucketName,
				artifactKey:        conf.ArtifactKey,
				rc:                 conf.RuntimeConfig,
				image:              conf.Manifest.ImageConfig.Image,
				rawManifest:        conf.RawManifest,
				parser:             fs,
				addons:             conf.Addons,
			},
			logging:             conf.Manifest.Logging,
			sidecars:            conf.Manifest.Sidecars,
			tc:                  conf.Manifest.TaskConfig,
			taskDefOverrideFunc: override.CloudFormationTemplate,
		},
		manifest:             conf.Manifest,
		httpsEnabled:         httpsEnabled,
		appInfo:              appInfo,
		dnsDelegationEnabled: dnsDelegationEnabled,

		parser: fs,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *LoadBalancedWebService) Template() (string, error) {
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
	nlbConfig, err := s.convertNetworkLoadBalancer()
	if err != nil {
		return "", err
	}
	albListenerConfig, err := s.convertALBListener()
	if err != nil {
		return "", err
	}
	importedALBConfig, err := s.convertImportedALB()
	if err != nil {
		return "", err
	}
	scTarget := s.manifest.ServiceConnectTarget(exposedPorts)
	scOpts := template.ServiceConnectOpts{
		Server: convertServiceConnectServer(s.manifest.Network.Connect, scTarget),
		Client: s.manifest.Network.Connect.Enabled(),
	}

	// Set container-level feature flag.
	logConfig := convertLogging(s.manifest.Logging)
	content, err := s.parser.ParseLoadBalancedWebService(template.WorkloadOpts{
		// Workload parameters.
		AppName:            s.app,
		EnvName:            s.env,
		EnvVersion:         s.rc.EnvVersion,
		Version:            s.rc.Version,
		SerializedManifest: string(s.rawManifest),
		WorkloadName:       s.name,
		WorkloadType:       manifestinfo.LoadBalancedWebServiceType,

		// Configuration for the main container.
		Command:      command,
		EntryPoint:   entrypoint,
		HealthCheck:  convertContainerHealthCheck(s.manifest.ImageConfig.HealthCheck),
		PortMappings: convertPortMappings(exposedPorts.PortsForContainer[s.name]),
		Secrets:      convertSecrets(s.manifest.TaskConfig.Secrets),
		Variables:    convertEnvVars(s.manifest.TaskConfig.Variables),

		// Additional options that are common between **all** workload templates.
		AddonsExtraParams:       addonsParams,
		Autoscaling:             autoscaling,
		CapacityProviders:       capacityProviders,
		CredentialsParameter:    aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		DesiredCountOnSpot:      desiredCountOnSpot,
		DeploymentConfiguration: convertDeploymentConfig(s.manifest.DeployConfig),
		DependsOn:               convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		DockerLabels:            s.manifest.ImageConfig.Image.DockerLabels,
		ExecuteCommand:          convertExecuteCommand(&s.manifest.ExecuteCommand),
		LogConfig:               logConfig,
		NestedStack:             addonsOutputs,
		Network:                 convertNetworkConfig(s.manifest.Network),
		Publish:                 publishers,
		PermissionsBoundary:     s.permBound,
		Platform:                convertPlatform(s.manifest.Platform),
		Storage:                 convertStorageOpts(s.manifest.Name, s.manifest.Storage),

		// ALB configs.
		ALBEnabled:  !s.manifest.HTTPOrBool.Disabled(),
		GracePeriod: s.convertGracePeriod(),
		ALBListener: albListenerConfig,
		ImportedALB: importedALBConfig,

		// NLB configs.
		AppDNSName:           nlbConfig.appDNSName,
		AppDNSDelegationRole: nlbConfig.appDNSDelegationRole,
		NLB:                  nlbConfig.settings,

		// service connect and service discovery options.
		ServiceConnectOpts:       scOpts,
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,

		// Additional options for request driven web service templates.
		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},

		// Sidecar configs.
		Sidecars: sidecars,

		// Custom Resource Config.
		CustomResources: crs,
	})
	if err != nil {
		return "", err
	}
	overriddenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overriddenTpl), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *LoadBalancedWebService) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	exposedPorts, err := s.manifest.ExposedPorts()
	if err != nil {
		return nil, fmt.Errorf("parse exposed ports in service manifest %s: %w", s.name, err)
	}
	targetContainer, targetPort, err := s.manifest.HTTPOrBool.Main.Target(exposedPorts)
	if err != nil {
		return nil, err
	}
	wkldParams = append(wkldParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String(s.manifest.MainContainerPort()),
		},
		{
			ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
			ParameterValue: aws.String(strconv.FormatBool(s.dnsDelegationEnabled)),
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

	if !s.manifest.HTTPOrBool.Disabled() {
		wkldParams = append(wkldParams, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(WorkloadRulePathParamKey),
				ParameterValue: s.manifest.HTTPOrBool.Main.Path,
			},
			{
				ParameterKey:   aws.String(WorkloadHTTPSParamKey),
				ParameterValue: aws.String(strconv.FormatBool(s.httpsEnabled)),
			},
		}...)
	}
	if !s.manifest.NLBConfig.IsEmpty() {
		port, _, err := manifest.ParsePortMapping(s.manifest.NLBConfig.Listener.Port)
		if err != nil {
			return nil, err
		}
		wkldParams = append(wkldParams, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(LBWebServiceNLBAliasesParamKey),
				ParameterValue: aws.String(s.manifest.NLBConfig.Aliases.ToString()),
			},
			{
				ParameterKey:   aws.String(LBWebServiceNLBPortParamKey),
				ParameterValue: port,
			},
		}...)
	}
	return wkldParams, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (s *LoadBalancedWebService) SerializedParameters() (string, error) {
	return serializeTemplateConfig(s.wkld.parser, s)
}
