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

	parser loadBalancedWebSvcReadParser
}

// NewLoadBalancedWebService creates a new LoadBalancedWebService stack from a manifest file.
func NewLoadBalancedWebService(mft *manifest.LoadBalancedWebService, env, app string, rc RuntimeConfig) (*LoadBalancedWebService, error) {
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
				image:  mft.ImageConfig,
				parser: parser,
				addons: addons,
			},
			tc: mft.TaskConfig,
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
	webSvc, err := NewLoadBalancedWebService(mft, env, app, rc)
	if err != nil {
		return nil, err
	}
	webSvc.httpsEnabled = true
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
	convSidecarOpts := convertSidecarOpts{
		sidecarConfig: s.manifest.Sidecars,
		imageConfig:   &s.manifest.ImageConfig.Image,
		workloadName:  aws.StringValue(s.manifest.Name),
	}
	sidecars, err := convertSidecar(convSidecarOpts)
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for service %s: %w", s.name, err)
	}
	dependencies, err := convertImageDependsOn(convSidecarOpts)
	if err != nil {
		return "", fmt.Errorf("convert the container dependency for service %s: %w", s.name, err)
	}
	publishers, err := convertPublish(s.manifest.Publish, s.rc.SNSTopicARNs)
	if err != nil {
		return "", fmt.Errorf("convert the publish field for service %s: %w", s.name, err)
	}

	advancedCount, err := convertAdvancedCount(&s.manifest.Count.AdvancedCount)
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

	storage, err := convertStorageOpts(s.manifest.Name, s.manifest.Storage)
	if err != nil {
		return "", fmt.Errorf("convert storage options for service %s: %w", s.name, err)
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
		albAlias := aws.StringValue(s.manifest.Alias)
		if albAlias != "" {
			aliases = append(aliases, albAlias)
		}
	}

	var deregistrationDelay *int64 = aws.Int64(60)
	if s.manifest.RoutingRule.DeregistrationDelay != nil {
		deregistrationDelay = aws.Int64(int64(s.manifest.RoutingRule.DeregistrationDelay.Seconds()))
	}

	var allowedSourceIPs []string
	if s.manifest.AllowedSourceIps != nil {
		allowedSourceIPs = *s.manifest.AllowedSourceIps
	}
	content, err := s.parser.ParseLoadBalancedWebService(template.WorkloadOpts{
		Variables:                s.manifest.Variables,
		Secrets:                  s.manifest.Secrets,
		Aliases:                  aliases,
		NestedStack:              outputs,
		Sidecars:                 sidecars,
		LogConfig:                convertLogging(s.manifest.Logging),
		DockerLabels:             s.manifest.ImageConfig.DockerLabels,
		Autoscaling:              autoscaling,
		CapacityProviders:        capacityProviders,
		DesiredCountOnSpot:       desiredCountOnSpot,
		ExecuteCommand:           convertExecuteCommand(&s.manifest.ExecuteCommand),
		WorkloadType:             manifest.LoadBalancedWebServiceType,
		HealthCheck:              s.manifest.ImageConfig.HealthCheckOpts(),
		HTTPHealthCheck:          convertHTTPHealthCheck(&s.manifest.HealthCheck),
		DeregistrationDelay:      deregistrationDelay,
		AllowedSourceIps:         allowedSourceIPs,
		RulePriorityLambda:       rulePriorityLambda.String(),
		DesiredCountLambda:       desiredCountLambda.String(),
		EnvControllerLambda:      envControllerLambda.String(),
		Storage:                  storage,
		Network:                  convertNetworkConfig(s.manifest.Network),
		EntryPoint:               entrypoint,
		Command:                  command,
		DependsOn:                dependencies,
		CredentialsParameter:     aws.StringValue(s.manifest.ImageConfig.Credentials),
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,
		Publish:                  publishers,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

func (s *LoadBalancedWebService) loadBalancerTarget() (targetContainer *string, targetPort *string, err error) {
	containerName := s.name
	containerPort := strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.ImageConfig.Port)), 10)
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(containerName)
	targetPort = aws.String(containerPort)
	if s.manifest.TargetContainer == nil && s.manifest.TargetContainerCamelCase != nil {
		s.manifest.TargetContainer = s.manifest.TargetContainerCamelCase
	}
	mftTargetContainer := s.manifest.TargetContainer
	if mftTargetContainer != nil {
		sidecar, ok := s.manifest.Sidecars[*mftTargetContainer]
		if ok {
			if sidecar.Port == nil {
				return nil, nil, fmt.Errorf("target container %s doesn't expose any port", *mftTargetContainer)
			}
			targetContainer = mftTargetContainer
			targetPort = sidecar.Port
		} else {
			return nil, nil, fmt.Errorf("target container %s doesn't exist", *mftTargetContainer)
		}
	}
	return
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *LoadBalancedWebService) Parameters() ([]*cloudformation.Parameter, error) {
	wkldParams, err := s.ecsWkld.Parameters()
	if err != nil {
		return nil, err
	}
	targetContainer, targetPort, err := s.loadBalancerTarget()
	if err != nil {
		return nil, err
	}
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
