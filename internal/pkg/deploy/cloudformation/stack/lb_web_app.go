// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Template rendering configuration.
const (
	lbWebAppTemplateName              = "lb-web-app"
	lbWebAppRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
)

// Parameter logical IDs unique for a load balanced web application.
const (
	LBWebAppHTTPSParamKey           = "HTTPSEnabled"
	LBWebAppContainerPortParamKey   = "ContainerPort"
	LBWebAppRulePathParamKey        = "RulePath"
	LBWebAppHealthCheckPathParamKey = "HealthCheckPath"
)

type templater interface {
	Template() (string, error)
}

// LoadBalancedWebApp represents the configuration needed to create a CloudFormation stack from a load balanced web application manifest.
type LoadBalancedWebApp struct {
	*app
	manifest     *manifest.LoadBalancedWebApp
	httpsEnabled bool

	addons templater
}

// NewLoadBalancedWebApp creates a new LoadBalancedWebApp stack from a manifest file.
// The stack is configured
func NewLoadBalancedWebApp(mft *manifest.LoadBalancedWebApp, env, proj string, rc RuntimeConfig) (*LoadBalancedWebApp, error) {
	parser := template.New()
	addons, err := addons.New(mft.Name)
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &LoadBalancedWebApp{
		app: &app{
			name:    mft.Name,
			env:     env,
			project: proj,
			tc:      mft.ApplyEnv(env).TaskConfig,
			rc:      rc,
			parser:  parser,
		},
		manifest:     mft,
		httpsEnabled: false,

		addons: addons,
	}, nil
}

// NewHTTPSLoadBalancedWebApp  creates a new LoadBalancedWebApp stack from its manifest that needs to be deployed to
// a environment within a project. It creates an HTTPS listener and assumes that the environment
// it's being deployed into has an HTTPS configured listener.
func NewHTTPSLoadBalancedWebApp(mft *manifest.LoadBalancedWebApp, env, proj string, rc RuntimeConfig) (*LoadBalancedWebApp, error) {
	parser := template.New()
	addons, err := addons.New(mft.Name)
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &LoadBalancedWebApp{
		app: &app{
			name:    mft.Name,
			env:     env,
			project: proj,
			tc:      mft.ApplyEnv(env).TaskConfig,
			rc:      rc,
			parser:  parser,
		},
		manifest:     mft,
		httpsEnabled: true,

		addons: addons,
	}, nil
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
	content, err := c.parser.ParseApp(lbWebAppTemplateName, struct {
		RulePriorityLambda string
		AddonsOutputs      []addons.Output
		*lbWebAppTemplateParams
	}{
		RulePriorityLambda:     rulePriorityLambda.String(),
		AddonsOutputs:          outputs,
		lbWebAppTemplateParams: c.toTemplateParams(),
	}, template.WithFuncs(map[string]interface{}{
		"toSnakeCase":           toSnakeCase,
		"filterSecrets":         filterSecrets,
		"filterManagedPolicies": filterManagedPolicies,
	}))
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (c *LoadBalancedWebApp) Parameters() []*cloudformation.Parameter {
	templateParams := c.toTemplateParams()
	return append(c.app.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBWebAppContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(templateParams.Image.Port), 10)),
		},
		{
			ParameterKey:   aws.String(LBWebAppRulePathParamKey),
			ParameterValue: aws.String(templateParams.App.Path),
		},
		{
			ParameterKey:   aws.String(LBWebAppHealthCheckPathParamKey),
			ParameterValue: aws.String(templateParams.App.HealthCheckPath),
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

func (c *LoadBalancedWebApp) addonsOutputs() ([]addons.Output, error) {
	stack, err := c.addons.Template()
	if err == nil {
		return addons.Outputs(stack)
	}

	var noAddonsErr *addons.ErrDirNotExist
	if !errors.As(err, &noAddonsErr) {
		return nil, fmt.Errorf("generate addons template for application %s: %w", c.manifest.Name, err)
	}
	return nil, nil // Addons directory does not exist, so there are no outputs and error.
}

// lbWebAppTemplateParams holds the data to render the CloudFormation template for an application.
type lbWebAppTemplateParams struct {
	App *manifest.LoadBalancedWebApp
	Env *archer.Environment

	HTTPSEnabled string
	// Field types to override.
	Image struct {
		URL  string
		Port uint16
	}
}

func (c *LoadBalancedWebApp) toTemplateParams() *lbWebAppTemplateParams {
	url := fmt.Sprintf("%s:%s", c.rc.ImageRepoURL, c.rc.ImageTag)
	return &lbWebAppTemplateParams{
		App: &manifest.LoadBalancedWebApp{
			App:                      c.manifest.App,
			LoadBalancedWebAppConfig: c.manifest.ApplyEnv(c.env), // Get environment specific app configuration.
		},
		Env: &archer.Environment{
			Name:    c.env,
			Project: c.project,
		},
		HTTPSEnabled: strconv.FormatBool(c.httpsEnabled),
		Image: struct {
			URL  string
			Port uint16
		}{
			URL:  url,
			Port: c.manifest.Image.Port,
		},
	}
}
