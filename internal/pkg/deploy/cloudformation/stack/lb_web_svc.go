// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
)

// Template rendering configuration.
const (
	lbWebSvcRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
	desiredCountGeneratorPath         = "custom-resources/desired-count-delegation.js"
	envControllerPath                 = "custom-resources/env-controller.js"
	nlbCertManagerPath                = "custom-resources/nlb-cert-validator-updater.js"
)

// Parameter logical IDs for a load balanced web service.
const (
	LBWebServiceHTTPSParamKey           = "HTTPSEnabled"
	LBWebServiceContainerPortParamKey   = "ContainerPort"
	LBWebServiceRulePathParamKey        = "RulePath"
	LBWebServiceTargetContainerParamKey = "TargetContainer"
	LBWebServiceTargetPortParamKey      = "TargetPort"
	LBWebServiceStickinessParamKey      = "Stickiness"
	LBWebServiceDNSDelegatedParamKey    = "DNSDelegated"
)

type loadBalancedWebSvcReadParser interface {
	template.ReadParser
	ParseLoadBalancedWebService(template.WorkloadOpts) (*template.Content, error)
}

// LoadBalancedWebService represents the configuration needed to create a CloudFormation stack from a load balanced web service manifest.
type LoadBalancedWebService struct {
	*ecsWkld
	manifest     *manifest.LoadBalancedWebService
	httpsEnabled bool

	// Fields for LoadBalancedWebService that needs a Network Load Balancer.

	// dnsDelegationEnabled is true if the application is associated with a domain. When an ALB is enabled,
	// `httpsEnabled` has the same value with `dnsDelegationEnabled`, because we enabled https
	// automatically the app is associated with a domain. When an ALB is disabled, `httpsEnabled`
	// should always be false; hence they could have different values at this time.
	dnsDelegationEnabled   bool
	publicSubnetCIDRBlocks []string
	appInfo                deploy.AppInformation

	parser loadBalancedWebSvcReadParser
}

// LoadBalancedWebServiceOption represents an option to apply to a LoadBalancedWebService.
type LoadBalancedWebServiceOption func(s *LoadBalancedWebService)

// WithHTTPS enables HTTPS for a LoadBalancedWebService. It creates an HTTPS listener and assumes that the environment
// the service is being deployed into has an HTTPS configured listener.
func WithHTTPS() func(s *LoadBalancedWebService) {
	return func(s *LoadBalancedWebService) {
		s.dnsDelegationEnabled = true
		s.httpsEnabled = true
	}
}

// WithNLB enables Network Load Balancer in a LoadBalancedWebService.
func WithNLB(cidrBlocks []string) func(s *LoadBalancedWebService) {
	return func(s *LoadBalancedWebService) {
		s.publicSubnetCIDRBlocks = cidrBlocks
	}
}

// WithDNSDelegation enables DNS delegation for a LoadBalancedWebService.
func WithDNSDelegation(app deploy.AppInformation) func(s *LoadBalancedWebService) {
	return func(s *LoadBalancedWebService) {
		s.dnsDelegationEnabled = true
		s.appInfo = app
	}
}

// NewLoadBalancedWebService creates a new CFN stack with an ECS service from a manifest file, given the options.
func NewLoadBalancedWebService(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig, opts ...LoadBalancedWebServiceOption) (*LoadBalancedWebService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	s := &LoadBalancedWebService{
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
		manifest:     mft,
		httpsEnabled: false,

		parser: parser,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *LoadBalancedWebService) Template() (string, error) {
	rulePriorityLambda, err := s.parser.Read(lbWebSvcRulePriorityGeneratorPath)
	if err != nil {
		return "", fmt.Errorf("read rule priority lambda: %w", err)
	}
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

	var aliases []string
	if s.httpsEnabled {
		if aliases, err = convertAlias(s.manifest.RoutingRule.Alias); err != nil {
			return "", err
		}
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
	content, err := s.parser.ParseLoadBalancedWebService(template.WorkloadOpts{
		Variables:                    s.manifest.TaskConfig.Variables,
		Secrets:                      s.manifest.TaskConfig.Secrets,
		Aliases:                      aliases,
		NestedStack:                  addonsOutputs,
		AddonsExtraParams:            addonsParams,
		Sidecars:                     sidecars,
		LogConfig:                    convertLogging(s.manifest.Logging),
		DockerLabels:                 s.manifest.ImageConfig.Image.DockerLabels,
		Autoscaling:                  autoscaling,
		CapacityProviders:            capacityProviders,
		DesiredCountOnSpot:           desiredCountOnSpot,
		ExecuteCommand:               convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:                 manifest.LoadBalancedWebServiceType,
		HealthCheck:                  convertContainerHealthCheck(s.manifest.ImageConfig.HealthCheck),
		HTTPHealthCheck:              convertHTTPHealthCheck(&s.manifest.RoutingRule.HealthCheck),
		DeregistrationDelay:          deregistrationDelay,
		AllowedSourceIps:             allowedSourceIPs,
		RulePriorityLambda:           rulePriorityLambda.String(),
		DesiredCountLambda:           desiredCountLambda.String(),
		EnvControllerLambda:          envControllerLambda.String(),
		NLBCertManagerFunctionLambda: nlbConfig.certManagerLambda,
		Storage:                      convertStorageOpts(s.manifest.Name, s.manifest.Storage),
		Network:                      convertNetworkConfig(s.manifest.Network),
		EntryPoint:                   entrypoint,
		Command:                      command,
		DependsOn:                    convertDependsOn(s.manifest.ImageConfig.Image.DependsOn),
		CredentialsParameter:         aws.StringValue(s.manifest.ImageConfig.Image.Credentials),
		ServiceDiscoveryEndpoint:     s.rc.ServiceDiscoveryEndpoint,
		Publish:                      publishers,
		Platform:                     convertPlatform(s.manifest.Platform),
		HTTPVersion:                  convertHTTPVersion(s.manifest.RoutingRule.ProtocolVersion),
		NLB:                          nlbConfig.settings,
		AppDNSName:                   nlbConfig.appDNSName,
		AppDNSDelegationRole:         nlbConfig.appDNSDelegationRole,
		HTTPDisabled:                 s.manifest.RoutingRule.Disabled(),
	})
	if err != nil {
		return "", err
	}
	overridenTpl, err := s.taskDefOverrideFunc(convertTaskDefOverrideRules(s.manifest.TaskDefOverrides), content.Bytes())
	if err != nil {
		return "", fmt.Errorf("apply task definition overrides: %w", err)
	}
	return string(overridenTpl), nil
}

func (s *LoadBalancedWebService) httpLoadBalancerTarget() (targetContainer *string, targetPort *string) {
	containerName := s.name
	containerPort := strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.ImageConfig.Port)), 10)
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(containerName)
	targetPort = aws.String(containerPort)
	if s.manifest.RoutingRule.TargetContainer != nil {
		targetContainer = s.manifest.RoutingRule.TargetContainer
	}
	if s.manifest.RoutingRule.TargetContainerCamelCase != nil {
		targetContainer = s.manifest.RoutingRule.TargetContainerCamelCase
	}
	if aws.StringValue(targetContainer) != containerName {
		targetPort = s.manifest.Sidecars[aws.StringValue(targetContainer)].Port
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
	targetContainer, targetPort := s.httpLoadBalancerTarget()
	return append(wkldParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBWebServiceContainerPortParamKey),
			ParameterValue: aws.String(s.containerPort()),
		},
		{
			ParameterKey:   aws.String(LBWebServiceRulePathParamKey),
			ParameterValue: s.manifest.RoutingRule.Path,
		},
		{
			ParameterKey:   aws.String(LBWebServiceHTTPSParamKey),
			ParameterValue: aws.String(strconv.FormatBool(s.httpsEnabled)),
		},
		{
			ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
			ParameterValue: aws.String(strconv.FormatBool(s.dnsDelegated())),
		},
		{
			ParameterKey:   aws.String(LBWebServiceTargetContainerParamKey),
			ParameterValue: targetContainer,
		},
		{
			ParameterKey:   aws.String(LBWebServiceTargetPortParamKey),
			ParameterValue: targetPort,
		},
		{
			ParameterKey:   aws.String(LBWebServiceStickinessParamKey),
			ParameterValue: aws.String(strconv.FormatBool(aws.BoolValue(s.manifest.RoutingRule.Stickiness))),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(s.rc.EnvFileARN),
		},
	}...), nil
}

func (s *LoadBalancedWebService) dnsDelegated() bool {
	return s.dnsDelegationEnabled || s.httpsEnabled
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *LoadBalancedWebService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
