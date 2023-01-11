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
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// Parameter logical IDs for a load balanced web service.
const (
	LBWebServiceDNSDelegatedParamKey = "DNSDelegated"
	LBWebServiceNLBAliasesParamKey   = "NLBAliases"
	LBWebServiceNLBPortParamKey      = "NLBPort"
)

type loadBalancedWebSvcReadParser interface {
	template.ReadParser
	ParseLoadBalancedWebService(template.WorkloadOpts) (*template.Content, error)
}

// LoadBalancedWebService represents the configuration needed to create a CloudFormation stack from a load balanced web service manifest.
type LoadBalancedWebService struct {
	*ecsWkld
	manifest               *manifest.LoadBalancedWebService
	httpsEnabled           bool
	dnsDelegationEnabled   bool
	publicSubnetCIDRBlocks []string
	appInfo                deploy.AppInformation

	parser               loadBalancedWebSvcReadParser
	EnvAddonsFeatureFlag bool
}

// LoadBalancedWebServiceOption is used to configuring an optional field for LoadBalancedWebService.
type LoadBalancedWebServiceOption func(s *LoadBalancedWebService)

// WithNLB enables Network Load Balancer in a LoadBalancedWebService.
func WithNLB(cidrBlocks []string) func(s *LoadBalancedWebService) {
	return func(s *LoadBalancedWebService) {
		s.publicSubnetCIDRBlocks = cidrBlocks
	}
}

// LoadBalancedWebServiceConfig contains fields to configure LoadBalancedWebService.
type LoadBalancedWebServiceConfig struct {
	App           *config.Application
	EnvManifest   *manifest.Environment
	Manifest      *manifest.LoadBalancedWebService
	RawManifest   []byte // Content of the manifest file without any transformations.
	RuntimeConfig RuntimeConfig
	RootUserARN   string
	Addons        NestedStackConfigurer
}

// NewLoadBalancedWebService creates a new CFN stack with an ECS service from a manifest file, given the options.
func NewLoadBalancedWebService(conf LoadBalancedWebServiceConfig,
	opts ...LoadBalancedWebServiceOption) (*LoadBalancedWebService, error) {
	parser := template.New()
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
	if conf.Manifest.RoutingRule.Disabled() {
		httpsEnabled = false
	}
	s := &LoadBalancedWebService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name:        aws.StringValue(conf.Manifest.Name),
				env:         aws.StringValue(conf.EnvManifest.Name),
				app:         conf.App.Name,
				permBound:   conf.App.PermissionsBoundary,
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
		manifest:             conf.Manifest,
		httpsEnabled:         httpsEnabled,
		appInfo:              appInfo,
		dnsDelegationEnabled: dnsDelegationEnabled,

		parser: parser,
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
		return "", fmt.Errorf("exposed ports configuration for service %s: %w", s.name, err)
	}
	portMappings := convertPortMapping(exposedPorts)
	sidecars, err := convertSidecar(s.manifest.Sidecars, portMappings)
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

	aliasesFor, err := convertHostedZone(s.manifest.RoutingRule.RoutingRuleConfiguration)
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
	nlbConfig, err := s.convertNetworkLoadBalancer()
	if err != nil {
		return "", err
	}
	httpRedirect := true
	if s.manifest.RoutingRule.RedirectToHTTPS != nil {
		httpRedirect = aws.BoolValue(s.manifest.RoutingRule.RedirectToHTTPS)
	}
	var scConfig *template.ServiceConnect
	if s.manifest.Network.Connect.Enabled() {
		scConfig = convertServiceConnect(s.manifest.Network.Connect)
	}
	targetContainer, targetContainerPort := s.httpLoadBalancerTarget(exposedPorts)

	// Set container-level feature flag.
	logConfig := convertLogging(s.manifest.Logging)
	if logConfig != nil {
		logConfig.EnvAddonsFeatureFlag = s.EnvAddonsFeatureFlag
	}
	for _, sidecar := range sidecars {
		if sidecar != nil {
			sidecar.EnvAddonsFeatureFlag = s.EnvAddonsFeatureFlag
		}
	}
	content, err := s.parser.ParseLoadBalancedWebService(template.WorkloadOpts{
		AppName:            s.app,
		EnvName:            s.env,
		WorkloadName:       s.name,
		SerializedManifest: string(s.rawManifest),
		EnvVersion:         s.rc.EnvVersion,

		Variables:          convertEnvVars(s.manifest.TaskConfig.Variables),
		Secrets:            convertSecrets(s.manifest.TaskConfig.Secrets),
		Aliases:            aliases,
		HTTPSListener:      s.httpsEnabled,
		HTTPRedirect:       httpRedirect,
		NestedStack:        addonsOutputs,
		AddonsExtraParams:  addonsParams,
		Sidecars:           sidecars,
		LogConfig:          logConfig,
		DockerLabels:       s.manifest.ImageConfig.Image.DockerLabels,
		Autoscaling:        autoscaling,
		CapacityProviders:  capacityProviders,
		DesiredCountOnSpot: desiredCountOnSpot,
		ExecuteCommand:     convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:       manifest.LoadBalancedWebServiceType,
		HTTPTargetContainer: template.HTTPTargetContainer{
			Port: aws.StringValue(targetContainerPort),
			Name: aws.StringValue(targetContainer),
		},
		ServiceConnect:           scConfig,
		HealthCheck:              convertContainerHealthCheck(s.manifest.ImageConfig.HealthCheck),
		HTTPHealthCheck:          convertHTTPHealthCheck(&s.manifest.RoutingRule.HealthCheck),
		DeregistrationDelay:      deregistrationDelay,
		AllowedSourceIps:         allowedSourceIPs,
		CustomResources:          crs,
		Storage:                  convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                  convertNetworkConfig(s.manifest.Network),
		EntryPoint:               entrypoint,
		Command:                  command,
		DependsOn:                convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:     aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,
		Publish:                  publishers,
		Platform:                 convertPlatform(s.manifest.Platform),
		HTTPVersion:              convertHTTPVersion(s.manifest.RoutingRule.ProtocolVersion),
		NLB:                      nlbConfig.settings,
		DeploymentConfiguration:  convertDeploymentConfig(s.manifest.DeployConfig),
		AppDNSName:               nlbConfig.appDNSName,
		AppDNSDelegationRole:     nlbConfig.appDNSDelegationRole,
		ALBEnabled:               !s.manifest.RoutingRule.Disabled(),
		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},
		HostedZoneAliases:    aliasesFor,
		PermissionsBoundary:  s.permBound,
		EnvAddonsFeatureFlag: s.EnvAddonsFeatureFlag, // Feature flag for main container
		PortMappings:         portMappings[s.name],
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

func (s *LoadBalancedWebService) httpLoadBalancerTarget(exposedPorts []manifest.ExposedPort) (targetContainer *string, targetPort *string) {
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(s.name)
	targetPort = aws.String(s.containerPort())

	rrTarget := s.manifest.RoutingRule.GetTargetContainer()
	if rrTarget != nil && *rrTarget != *targetContainer {
		targetContainer = rrTarget
		targetPort = s.manifest.Sidecars[aws.StringValue(targetContainer)].Port
	}

	// Route load balancer traffic to the target_port if mentioned.
	if s.manifest.RoutingRule.TargetPort != nil {
		port := aws.Uint16Value(s.manifest.RoutingRule.TargetPort)
		targetPort = aws.String(strconv.FormatUint(uint64(port), 10))
		if containerName := findContainerNameGivenPort(port, exposedPorts); containerName != "" {
			targetContainer = aws.String(containerName)
		}
	}

	return
}

func (s *LoadBalancedWebService) containerPort() string {
	return strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.ImageConfig.Port)), 10)
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *LoadBalancedWebService) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	exposedPorts, err := s.manifest.ExposedPorts()
	if err != nil {
		return nil, err
	}
	targetContainer, targetPort := s.httpLoadBalancerTarget(exposedPorts)
	wkldParams = append(wkldParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String(s.containerPort()),
		},
		{
			ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
			ParameterValue: aws.String(strconv.FormatBool(s.dnsDelegationEnabled)),
		},
		{
			ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
			ParameterValue: targetContainer,
		},
		{
			ParameterKey:   aws.String(WorkloadTargetPortParamKey),
			ParameterValue: targetPort,
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(s.rc.EnvFileARN),
		},
	}...)

	if !s.manifest.RoutingRule.Disabled() {
		wkldParams = append(wkldParams, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(WorkloadRulePathParamKey),
				ParameterValue: s.manifest.RoutingRule.Path,
			},
			{
				ParameterKey:   aws.String(WorkloadHTTPSParamKey),
				ParameterValue: aws.String(strconv.FormatBool(s.httpsEnabled)),
			},
			{
				ParameterKey:   aws.String(WorkloadStickinessParamKey),
				ParameterValue: aws.String(strconv.FormatBool(aws.BoolValue(s.manifest.RoutingRule.Stickiness))),
			},
		}...)
	}
	if !s.manifest.NLBConfig.IsEmpty() {
		port, _, err := manifest.ParsePortMapping(s.manifest.NLBConfig.Port)
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
