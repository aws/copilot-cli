// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Template rendering configuration.
const (
	lbWebAppRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
)

// Parameter logical IDs for a load balanced web application.
const (
	LBWebAppHTTPSParamKey           = "HTTPSEnabled"
	LBWebAppContainerPortParamKey   = "ContainerPort"
	LBWebAppRulePathParamKey        = "RulePath"
	LBWebAppHealthCheckPathParamKey = "HealthCheckPath"
)

type loadBalancedWebAppReadParser interface {
	template.ReadParser
	ParseLoadBalancedWebService(template.ServiceOpts) (*template.Content, error)
}

// LoadBalancedWebApp represents the configuration needed to create a CloudFormation stack from a load balanced web application manifest.
type LoadBalancedWebApp struct {
	*app
	manifest     *manifest.LoadBalancedWebApp
	httpsEnabled bool

	parser loadBalancedWebAppReadParser
}

// NewLoadBalancedWebApp creates a new LoadBalancedWebApp stack from a manifest file.
func NewLoadBalancedWebApp(mft *manifest.LoadBalancedWebApp, env, proj string, rc RuntimeConfig) (*LoadBalancedWebApp, error) {
	parser := template.New()
	addons, err := addons.New(mft.Name)
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	return &LoadBalancedWebApp{
		app: &app{
			name:    mft.Name,
			env:     env,
			project: proj,
			tc:      envManifest.TaskConfig,
			rc:      rc,
			parser:  parser,
			addons:  addons,
		},
		manifest:     envManifest,
		httpsEnabled: false,

		parser: parser,
	}, nil
}

// NewHTTPSLoadBalancedWebApp  creates a new LoadBalancedWebApp stack from its manifest that needs to be deployed to
// a environment within a project. It creates an HTTPS listener and assumes that the environment
// it's being deployed into has an HTTPS configured listener.
func NewHTTPSLoadBalancedWebApp(mft *manifest.LoadBalancedWebApp, env, proj string, rc RuntimeConfig) (*LoadBalancedWebApp, error) {
	webApp, err := NewLoadBalancedWebApp(mft, env, proj, rc)
	if err != nil {
		return nil, err
	}
	webApp.httpsEnabled = true
	return webApp, nil
}

// Template returns the CloudFormation template for the application parametrized for the environment.
func (c *LoadBalancedWebApp) Template() (string, error) {
	rulePriorityLambda, err := c.parser.Read(lbWebAppRulePriorityGeneratorPath)
	if err != nil {
		return "", err
	}
	outputs, err := c.addonsOutputs()
	if err != nil {
		return "", err
	}
	content, err := c.parser.ParseLoadBalancedWebService(template.ServiceOpts{
		Variables:          c.manifest.Variables,
		Secrets:            c.manifest.Secrets,
		NestedStack:        outputs,
		RulePriorityLambda: rulePriorityLambda.String(),
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (c *LoadBalancedWebApp) Parameters() []*cloudformation.Parameter {
	return append(c.app.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBWebAppContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(c.manifest.Image.Port), 10)),
		},
		{
			ParameterKey:   aws.String(LBWebAppRulePathParamKey),
			ParameterValue: aws.String(c.manifest.Path),
		},
		{
			ParameterKey:   aws.String(LBWebAppHealthCheckPathParamKey),
			ParameterValue: aws.String(c.manifest.HealthCheckPath),
		},
		{
			ParameterKey:   aws.String(LBWebAppHTTPSParamKey),
			ParameterValue: aws.String(strconv.FormatBool(c.httpsEnabled)),
		},
	}...)
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (c *LoadBalancedWebApp) SerializedParameters() (string, error) {
	return c.app.templateConfiguration(c)
}
