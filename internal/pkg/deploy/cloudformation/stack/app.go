// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Template rendering configuration common across applications.
const (
	appParamsTemplatePath = "applications/params.json.tmpl"
)

// Parameter logical IDs common across applications.
const (
	AppProjectNameParamKey       = "ProjectName"
	AppEnvNameParamKey           = "EnvName"
	AppNameParamKey              = "AppName"
	AppContainerImageParamKey    = "ContainerImage"
	AppTaskCPUParamKey           = "TaskCPU"
	AppTaskMemoryParamKey        = "TaskMemory"
	AppTaskCountParamKey         = "TaskCount"
	AppLogRetentionParamKey      = "LogRetention"
	AppAddonsTemplateURLParamKey = "AddonsTemplateURL"
)

// RuntimeConfig represents configuration that's defined outside of the manifest file
// that is needed to create a CloudFormation stack.
type RuntimeConfig struct {
	ImageRepoURL   string            // ImageRepoURL is the ECR repository URL the container image should be pushed to.
	ImageTag       string            // ImageTag is the container image's unique tag.
	AdditionalTags map[string]string // AdditionalTags are labels applied to resources in the application stack.
}

type app struct {
	name    string
	env     string
	project string
	tc      manifest.TaskConfig
	rc      RuntimeConfig

	parser template.AppTemplateReadParser
}

// StackName returns the name of the stack.
func (a *app) StackName() string {
	return NameForApp(a.project, a.env, a.name)
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (a *app) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(AppProjectNameParamKey),
			ParameterValue: aws.String(a.project),
		},
		{
			ParameterKey:   aws.String(AppEnvNameParamKey),
			ParameterValue: aws.String(a.env),
		},
		{
			ParameterKey:   aws.String(AppNameParamKey),
			ParameterValue: aws.String(a.name),
		},
		{
			ParameterKey:   aws.String(AppContainerImageParamKey),
			ParameterValue: aws.String(fmt.Sprintf("%s:%s", a.rc.ImageRepoURL, a.rc.ImageTag)),
		},
		{
			ParameterKey:   aws.String(AppTaskCPUParamKey),
			ParameterValue: aws.String(strconv.Itoa(a.tc.CPU)),
		},
		{
			ParameterKey:   aws.String(AppTaskMemoryParamKey),
			ParameterValue: aws.String(strconv.Itoa(a.tc.Memory)),
		},
		{
			ParameterKey:   aws.String(AppTaskCountParamKey),
			ParameterValue: aws.String(strconv.Itoa(*a.tc.Count)),
		},
		{
			ParameterKey:   aws.String(AppLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(AppAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
	}
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (a *app) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(a.rc.AdditionalTags, map[string]string{
		ProjectTagKey: a.project,
		EnvTagKey:     a.env,
		AppTagKey:     a.name,
	})
}

type templateConfigurer interface {
	Parameters() []*cloudformation.Parameter
	Tags() []*cloudformation.Tag
}

func (a *app) templateConfiguration(tc templateConfigurer) (string, error) {
	doc, err := a.parser.Parse(appParamsTemplatePath, struct {
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
