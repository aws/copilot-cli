// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addon"
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
func (s *svc) StackName() string {
	return NameForService(s.app, s.env, s.name)
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *svc) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(ServiceAppNameParamKey),
			ParameterValue: aws.String(s.app),
		},
		{
			ParameterKey:   aws.String(ServiceEnvNameParamKey),
			ParameterValue: aws.String(s.env),
		},
		{
			ParameterKey:   aws.String(ServiceNameParamKey),
			ParameterValue: aws.String(s.name),
		},
		{
			ParameterKey:   aws.String(ServiceContainerImageParamKey),
			ParameterValue: aws.String(fmt.Sprintf("%s:%s", s.rc.ImageRepoURL, s.rc.ImageTag)),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(s.tc.CPU))),
		},
		{
			ParameterKey:   aws.String(ServiceTaskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(aws.IntValue(s.tc.Memory))),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCountParamKey),
			ParameterValue: aws.String(strconv.Itoa(*s.tc.Count)),
		},
		{
			ParameterKey:   aws.String(ServiceLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(ServiceAddonsTemplateURLParamKey),
			ParameterValue: aws.String(s.rc.AddonsTemplateURL),
		},
	}
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (s *svc) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(s.rc.AdditionalTags, map[string]string{
		AppTagKey:     s.app,
		EnvTagKey:     s.env,
		ServiceTagKey: s.name,
	})
}

type templateConfigurer interface {
	Parameters() ([]*cloudformation.Parameter, error)
	Tags() []*cloudformation.Tag
}

func (s *svc) templateConfiguration(tc templateConfigurer) (string, error) {
	params, err := tc.Parameters()
	if err != nil {
		return "", err
	}
	doc, err := s.parser.Parse(svcParamsTemplatePath, struct {
		Parameters []*cloudformation.Parameter
		Tags       []*cloudformation.Tag
	}{
		Parameters: params,
		Tags:       tc.Tags(),
	}, template.WithFuncs(map[string]interface{}{
		"inc": template.IncFunc,
	}))
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}

func (s *svc) addonsOutputs() (*template.ServiceNestedStackOpts, error) {
	stack, err := s.addons.Template()
	if err != nil {
		var noAddonsErr *addon.ErrDirNotExist
		if !errors.As(err, &noAddonsErr) {
			return nil, fmt.Errorf("generate addons template for service %s: %w", s.name, err)
		}
		return nil, nil // Addons directory does not exist, so there are no outputs and error.
	}

	out, err := addon.Outputs(stack)
	if err != nil {
		return nil, fmt.Errorf("get addons outputs for service %s: %w", s.name, err)
	}
	return &template.ServiceNestedStackOpts{
		StackName:       addon.StackName,
		VariableOutputs: envVarOutputNames(out),
		SecretOutputs:   secretOutputNames(out),
		PolicyOutputs:   managedPolicyOutputNames(out),
	}, nil
}

func secretOutputNames(outputs []addon.Output) []string {
	var secrets []string
	for _, out := range outputs {
		if out.IsSecret {
			secrets = append(secrets, out.Name)
		}
	}
	return secrets
}

func managedPolicyOutputNames(outputs []addon.Output) []string {
	var policies []string
	for _, out := range outputs {
		if out.IsManagedPolicy {
			policies = append(policies, out.Name)
		}
	}
	return policies
}

func envVarOutputNames(outputs []addon.Output) []string {
	var envVars []string
	for _, out := range outputs {
		if !out.IsSecret && !out.IsManagedPolicy {
			envVars = append(envVars, out.Name)
		}
	}
	return envVars
}
