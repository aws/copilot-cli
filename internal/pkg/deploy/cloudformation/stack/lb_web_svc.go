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

// Template rendering configuration.
const (
	lbWebSvcRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
	desiredCountGeneratorPath         = "custom-resources/desired-count-delegation.js"
	envControllerPath                 = "custom-resources/env-controller.js"
)

// Parameter logical IDs for a load balanced web service.
const (
	LBWebServiceHTTPSParamKey           = "HTTPSEnabled"
	LBWebServiceContainerPortParamKey   = "ContainerPort"
	LBWebServiceRulePathParamKey        = "RulePath"
	LBWebServiceTargetContainerParamKey = "TargetContainer"
	LBWebServiceTargetPortParamKey      = "TargetPort"
	LBWebServiceStickinessParamKey      = "Stickiness"
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
	dnsDelegationEnabled   bool
	publicSubnetCIDRBlocks []string

	parser loadBalancedWebSvcReadParser
}

// NewHTTPLoadBalancedWebService creates a new LoadBalancedWebService stack from a manifest file.
func NewHTTPLoadBalancedWebService(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig) (*LoadBalancedWebService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &LoadBalancedWebService{
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
	}, nil
}

// NewHTTPSLoadBalancedWebService  creates a new LoadBalancedWebService stack from its manifest that needs to be deployed to
// a environment within an application. It creates an HTTPS listener and assumes that the environment
// it's being deployed into has an HTTPS configured listener.
func NewHTTPSLoadBalancedWebService(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig) (*LoadBalancedWebService, error) {
	webSvc, err := NewHTTPLoadBalancedWebService(mft, env, app, rc)
	if err != nil {
		return nil, err
	}
	webSvc.httpsEnabled = true
	return webSvc, nil
}

// NewNetworkLoadBalancedWebService creates a new LoadBalancedWebService stack from a manifest file. It creates a
// Network Load Balancer, with or without an Application Load Balancer.
func NewNetworkLoadBalancedWebService(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig, cidrBlocks []string) (*LoadBalancedWebService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &LoadBalancedWebService{
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
		manifest:               mft,
		publicSubnetCIDRBlocks: cidrBlocks,

		parser: parser,
	}, nil
}

// NewNetworkLoadBalancedWebServiceWithDNS creates a new LoadBalancedWebService stack from a manifest file. It creates a
// Network Load Balancer, with or without an Application Load Balancer, and use the delegated domain name to create a
// friendly alias for the Network Load Balancer.
func NewNetworkLoadBalancedWebServiceWithDNS(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig, cidrBlocks []string) (*LoadBalancedWebService, error) {
	webSvc, err := NewNetworkLoadBalancedWebService(mft, env, app, rc, cidrBlocks)
	if err != nil {
		return nil, err
	}
	webSvc.dnsDelegationEnabled = true
	return webSvc, nil
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
	outputs, err := s.addonsOutputs()
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
		if aliases, err = convertAlias(s.manifest.Alias); err != nil {
			return "", err
		}
	}

	var deregistrationDelay *int64 = aws.Int64(60)
	if s.manifest.RoutingRule.DeregistrationDelay != nil {
		deregistrationDelay = aws.Int64(int64(s.manifest.RoutingRule.DeregistrationDelay.Seconds()))
	}

	var allowedSourceIPs []string
	for _, ipNet := range s.manifest.AllowedSourceIps {
		allowedSourceIPs = append(allowedSourceIPs, string(ipNet))
	}

	nlb := convertNetworkLoadBalancer()
	content, err := s.parser.ParseLoadBalancedWebService(template.WorkloadOpts{
		Variables:                s.manifest.Variables,
		Secrets:                  s.manifest.Secrets,
		Aliases:                  aliases,
		NestedStack:              outputs,
		Sidecars:                 sidecars,
		LogConfig:                convertLogging(s.manifest.Logging),
		DockerLabels:             s.manifest.ImageConfig.Image.DockerLabels,
		Autoscaling:              autoscaling,
		CapacityProviders:        capacityProviders,
		DesiredCountOnSpot:       desiredCountOnSpot,
		ExecuteCommand:           convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:             manifest.LoadBalancedWebServiceType,
		HealthCheck:              convertContainerHealthCheck(s.manifest.ImageConfig.HealthCheck),
		HTTPHealthCheck:          convertHTTPHealthCheck(&s.manifest.HealthCheck),
		DeregistrationDelay:      deregistrationDelay,
		AllowedSourceIps:         allowedSourceIPs,
		RulePriorityLambda:       rulePriorityLambda.String(),
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
		NLB:                      nlb,
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

func (s *LoadBalancedWebService) loadBalancerTarget() (targetContainer *string, targetPort *string) {
	containerName := s.name
	containerPort := strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.ImageConfig.Port)), 10)
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(containerName)
	targetPort = aws.String(containerPort)
	if s.manifest.TargetContainer != nil {
		targetContainer = s.manifest.TargetContainer
	}
	if s.manifest.TargetContainerCamelCase != nil {
		targetContainer = s.manifest.TargetContainerCamelCase
	}
	if aws.StringValue(targetContainer) != containerName {
		targetPort = s.manifest.Sidecars[aws.StringValue(targetContainer)].Port
	}
	return
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *LoadBalancedWebService) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	targetContainer, targetPort := s.loadBalancerTarget()
	return append(wkldParams, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBWebServiceContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.ImageConfig.Port)), 10)),
		},
		{
			ParameterKey:   aws.String(LBWebServiceRulePathParamKey),
			ParameterValue: s.manifest.Path,
		},
		{
			ParameterKey:   aws.String(LBWebServiceHTTPSParamKey),
			ParameterValue: aws.String(strconv.FormatBool(s.httpsEnabled)),
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
			ParameterValue: aws.String(strconv.FormatBool(aws.BoolValue(s.manifest.Stickiness))),
		},
	}...), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *LoadBalancedWebService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
