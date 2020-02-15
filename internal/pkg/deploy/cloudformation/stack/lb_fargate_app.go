// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

const (
	lbFargateAppTemplatePath              = "lb-fargate-service/cf.yml"
	lbFargateAppParamsPath                = "lb-fargate-service/params.json"
	lbFargateAppRulePriorityGeneratorPath = "custom-resources/alb-rule-priority-generator.js"
)

// Parameter logical IDs for a load balanced Fargate service.
const (
	LBFargateParamProjectNameKey    = "ProjectName"
	LBFargateParamHTTPSKey          = "HTTPSEnabled"
	LBFargateParamEnvNameKey        = "EnvName"
	LBFargateParamAppNameKey        = "AppName"
	LBFargateParamContainerImageKey = "ContainerImage"
	LBFargateParamContainerPortKey  = "ContainerPort"
	LBFargateRulePathKey            = "RulePath"
	LBFargateTaskCPUKey             = "TaskCPU"
	LBFargateTaskMemoryKey          = "TaskMemory"
	LBFargateTaskCountKey           = "TaskCount"
)

// LBFargateStackConfig represents the configuration needed to create a CloudFormation stack from a
// load balanced Fargate application.
type LBFargateStackConfig struct {
	*deploy.CreateLBFargateAppInput
	httpsEnabled bool
	parser       template.ReadParser
}

// NewLBFargateStack creates a new LBFargateStackConfig from a load-balanced AWS Fargate application.
func NewLBFargateStack(in *deploy.CreateLBFargateAppInput) *LBFargateStackConfig {
	return &LBFargateStackConfig{
		CreateLBFargateAppInput: in,
		httpsEnabled:            false,
		parser:                  template.New(),
	}
}

// NewHTTPSLBFargateStack creates a new LBFargateStackConfig from a load-balanced AWS Fargate application. It
// creates an HTTPS listener and assumes that the environment it's being deployed into has an HTTPS configured
// listener.
func NewHTTPSLBFargateStack(in *deploy.CreateLBFargateAppInput) *LBFargateStackConfig {
	return &LBFargateStackConfig{
		CreateLBFargateAppInput: in,
		httpsEnabled:            true,
		parser:                  template.New(),
	}
}

// StackName returns the name of the stack.
func (c *LBFargateStackConfig) StackName() string {
	return NameForApp(c.Env.Project, c.Env.Name, c.App.Name)
}

// Template returns the CloudFormation template for the application parametrized for the environment.
func (c *LBFargateStackConfig) Template() (string, error) {
	rulePriorityLambda, err := c.parser.Read(lbFargateAppRulePriorityGeneratorPath)
	if err != nil {
		return "", err
	}
	content, err := c.parser.Parse(lbFargateAppTemplatePath, struct {
		RulePriorityLambda string
		*lbFargateTemplateParams
	}{
		RulePriorityLambda:      rulePriorityLambda.String(),
		lbFargateTemplateParams: c.toTemplateParams(),
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (c *LBFargateStackConfig) Parameters() []*cloudformation.Parameter {
	templateParams := c.toTemplateParams()
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(LBFargateParamProjectNameKey),
			ParameterValue: aws.String(templateParams.Env.Project),
		},
		{
			ParameterKey:   aws.String(LBFargateParamEnvNameKey),
			ParameterValue: aws.String(templateParams.Env.Name),
		},
		{
			ParameterKey:   aws.String(LBFargateParamAppNameKey),
			ParameterValue: aws.String(templateParams.App.Name),
		},
		{
			ParameterKey:   aws.String(LBFargateParamContainerImageKey),
			ParameterValue: aws.String(templateParams.Image.URL),
		},
		{
			ParameterKey:   aws.String(LBFargateParamContainerPortKey),
			ParameterValue: aws.String(strconv.Itoa(templateParams.Image.Port)),
		},
		{
			ParameterKey:   aws.String(LBFargateRulePathKey),
			ParameterValue: aws.String(templateParams.App.Path),
		},
		{
			ParameterKey:   aws.String(LBFargateTaskCPUKey),
			ParameterValue: aws.String(strconv.Itoa(templateParams.App.CPU)),
		},
		{
			ParameterKey:   aws.String(LBFargateTaskMemoryKey),
			ParameterValue: aws.String(strconv.Itoa(templateParams.App.Memory)),
		},
		{
			ParameterKey:   aws.String(LBFargateTaskCountKey),
			ParameterValue: aws.String(strconv.Itoa(templateParams.App.Count)),
		},
		{
			ParameterKey:   aws.String(LBFargateParamHTTPSKey),
			ParameterValue: aws.String(strconv.FormatBool(c.httpsEnabled)),
		},
	}
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (c *LBFargateStackConfig) SerializedParameters() (string, error) {
	params, err := c.parser.Parse(lbFargateAppParamsPath, c.toTemplateParams())
	if err != nil {
		return "", err
	}
	return params.String(), nil
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (c *LBFargateStackConfig) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String(c.Env.Project),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String(c.Env.Name),
		},
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String(c.App.Name),
		},
	}
}

// lbFargateTemplateParams holds the data to render the CloudFormation template for an application.
type lbFargateTemplateParams struct {
	*deploy.CreateLBFargateAppInput

	HTTPSEnabled string
	// Field types to override.
	Image struct {
		URL  string
		Port int
	}
}

func (c *LBFargateStackConfig) toTemplateParams() *lbFargateTemplateParams {
	url := fmt.Sprintf("%s:%s", c.ImageRepoURL, c.ImageTag)
	return &lbFargateTemplateParams{
		CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
			App: &manifest.LBFargateManifest{
				AppManifest:     c.App.AppManifest,
				LBFargateConfig: c.CreateLBFargateAppInput.App.EnvConf(c.Env.Name), // Get environment specific app configuration.
			},
			Env: c.Env,
		},
		HTTPSEnabled: strconv.FormatBool(c.httpsEnabled),
		Image: struct {
			URL  string
			Port int
		}{
			URL:  url,
			Port: c.App.Image.Port,
		},
	}
}
