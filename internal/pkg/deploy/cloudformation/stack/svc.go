// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Template rendering configuration common across services.
const (
	svcParamsTemplatePath = "services/params.json.tmpl"
)

// Parameter logical IDs common across services.
const (
	ServiceAppNameParamKey           = "AppName"
	ServiceEnvNameParamKey           = "EnvName"
	ServiceNameParamKey              = "ServiceName"
	ServiceContainerImageParamKey    = "ContainerImage"
	ServiceTaskCPUParamKey           = "TaskCPU"
	ServiceTaskMemoryParamKey        = "TaskMemory"
	ServiceTaskCountParamKey         = "TaskCount"
	ServiceLogRetentionParamKey      = "LogRetention"
	ServiceAddonsTemplateURLParamKey = "AddonsTemplateURL"
)

// RuntimeConfig represents configuration that's defined outside of the manifest file
// that is needed to create a CloudFormation stack.
type RuntimeConfig struct {
	ImageRepoURL      string            // ImageRepoURL is the ECR repository URL the container image should be pushed to.
	ImageTag          string            // ImageTag is the container image's unique tag.
	AddonsTemplateURL string            // Optional. S3 object URL for the addons template.
	AdditionalTags    map[string]string // AdditionalTags are labels applied to resources in the service stack.
}

type templater interface {
	Template() (string, error)
}

type svc struct {
	name string
	env  string
	app  string
	tc   manifest.TaskConfig
	rc   RuntimeConfig

	parser template.Parser
	addons templater
}

// StackName returns the name of the stack.
func (a *svc) StackName() string {
	return NameForService(a.app, a.env, a.name)
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (a *svc) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(ServiceAppNameParamKey),
			ParameterValue: aws.String(a.app),
		},
		{
			ParameterKey:   aws.String(ServiceEnvNameParamKey),
			ParameterValue: aws.String(a.env),
		},
		{
			ParameterKey:   aws.String(ServiceNameParamKey),
			ParameterValue: aws.String(a.name),
		},
		{
			ParameterKey:   aws.String(ServiceContainerImageParamKey),
			ParameterValue: aws.String(fmt.Sprintf("%s:%s", a.rc.ImageRepoURL, a.rc.ImageTag)),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(a.tc.CPU)),
		},
		{
			ParameterKey:   aws.String(ServiceTaskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(a.tc.Memory)),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCountParamKey),
			ParameterValue: aws.String(strconv.Itoa(*a.tc.Count)),
		},
		{
			ParameterKey:   aws.String(ServiceLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(ServiceAddonsTemplateURLParamKey),
			ParameterValue: aws.String(a.rc.AddonsTemplateURL),
		},
	}
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (a *svc) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(a.rc.AdditionalTags, map[string]string{
		AppTagKey:     a.app,
		EnvTagKey:     a.env,
		ServiceTagKey: a.name,
	})
}

type templateConfigurer interface {
	Parameters() []*cloudformation.Parameter
	Tags() []*cloudformation.Tag
}

func (a *svc) templateConfiguration(tc templateConfigurer) (string, error) {
	doc, err := a.parser.Parse(svcParamsTemplatePath, struct {
		Parameters []*cloudformation.Parameter
		Tags       []*cloudformation.Tag
	}{
		Parameters: tc.Parameters(),
		Tags:       tc.Tags(),
	}, template.WithFuncs(map[string]interface{}{
		"inc": func(i int) int { return i + 1 },
	}))
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}

func (a *svc) addonsOutputs() (*template.ServiceNestedStackOpts, error) {
	stack, err := a.addons.Template()
	if err != nil {
		var noAddonsErr *addons.ErrDirNotExist
		if !errors.As(err, &noAddonsErr) {
			return nil, fmt.Errorf("generate addons template for service %s: %w", a.name, err)
		}
		return nil, nil // Addons directory does not exist, so there are no outputs and error.
	}

	out, err := addons.Outputs(stack)
	if err != nil {
		return nil, fmt.Errorf("get addons outputs for service %s: %w", a.name, err)
	}
	return &template.ServiceNestedStackOpts{
		StackName:       addons.StackName,
		VariableOutputs: envVarOutputNames(out),
		SecretOutputs:   secretOutputNames(out),
		PolicyOutputs:   managedPolicyOutputNames(out),
	}, nil
}
