// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addon"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Template rendering configuration.
const (
	lbWebSvcRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
)

// Parameter logical IDs for a load balanced web service.
const (
	LBWebServiceHTTPSParamKey           = "HTTPSEnabled"
	LBWebServiceContainerPortParamKey   = "ContainerPort"
	LBWebServiceRulePathParamKey        = "RulePath"
	LBWebServiceHealthCheckPathParamKey = "HealthCheckPath"
	LBWebServiceTargetContainerParamKey = "TargetContainer"
	LBWebServiceTargetPortParamKey      = "TargetPort"
)

type loadBalancedWebSvcReadParser interface {
	template.ReadParser
	ParseLoadBalancedWebService(template.ServiceOpts) (*template.Content, error)
}

// LoadBalancedWebService represents the configuration needed to create a CloudFormation stack from a load balanced web service manifest.
type LoadBalancedWebService struct {
	*svc
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
	envManifest, err := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", env, err)
	}
	return &LoadBalancedWebService{
		svc: &svc{
			name:   aws.StringValue(mft.Name),
			env:    env,
			app:    app,
			tc:     envManifest.TaskConfig,
			rc:     rc,
			parser: parser,
			addons: addons,
		},
		manifest:     envManifest,
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
		return "", err
	}
	outputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	sidecars, err := s.manifest.Sidecar.SidecarsOpts()
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for service %s: %w", s.name, err)
	}
	content, err := s.parser.ParseLoadBalancedWebService(template.ServiceOpts{
		Variables:          s.manifest.Variables,
		Secrets:            s.manifest.Secrets,
		NestedStack:        outputs,
		Sidecars:           sidecars,
		LogConfig:          s.manifest.LogConfigOpts(),
		RulePriorityLambda: rulePriorityLambda.String(),
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

func (s *LoadBalancedWebService) loadBalancerTarget() (targetContainer *string, targetPort *string, err error) {
	containerName := s.name
	containerPort := strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.Image.Port)), 10)
	// Route load balancer traffic to main container by default.
	targetContainer = aws.String(containerName)
	targetPort = aws.String(containerPort)
	mftTargetContainer := s.manifest.TargetContainer
	if mftTargetContainer != nil {
		sidecar, ok := s.manifest.Sidecars[*mftTargetContainer]
		if ok {
			targetContainer = mftTargetContainer
			targetPort = sidecar.Port
		} else {
			return nil, nil, fmt.Errorf("target container %s doesn't exist", *s.manifest.TargetContainer)
		}
	}
	return
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *LoadBalancedWebService) Parameters() ([]*cloudformation.Parameter, error) {
	targetContainer, targetPort, err := s.loadBalancerTarget()
	if err != nil {
		return nil, err
	}
	return append(s.svc.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBWebServiceContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.Image.Port)), 10)),
		},
		{
			ParameterKey:   aws.String(LBWebServiceRulePathParamKey),
			ParameterValue: s.manifest.Path,
		},
		{
			ParameterKey:   aws.String(LBWebServiceHealthCheckPathParamKey),
			ParameterValue: s.manifest.HealthCheckPath,
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
	}...), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *LoadBalancedWebService) SerializedParameters() (string, error) {
	return s.svc.templateConfiguration(s)
}
