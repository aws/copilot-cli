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

// Parameter logical IDs for a backend application.
const (
	BackendAppContainerPortParamKey = "ContainerPort"
)

type backendAppReadParser interface {
	template.ReadParser
	ParseBackendService(template.ServiceOpts) (*template.Content, error)
}

// BackendApp represents the configuration needed to create a CloudFormation stack from a backend application manifest.
type BackendApp struct {
	*app
	manifest *manifest.BackendApp

	parser backendAppReadParser
}

// NewBackendApp creates a new BackendApp stack from a manifest file.
func NewBackendApp(mft *manifest.BackendApp, env, proj string, rc RuntimeConfig) (*BackendApp, error) {
	parser := template.New()
	addons, err := addons.New(mft.Name)
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	return &BackendApp{
		app: &app{
			name:    mft.Name,
			env:     env,
			project: proj,
			tc:      envManifest.TaskConfig,
			rc:      rc,
			parser:  parser,
			addons:  addons,
		},
		manifest: envManifest,

		parser: parser,
	}, nil
}

// Template returns the CloudFormation template for the backend application.
func (a *BackendApp) Template() (string, error) {
	outputs, err := a.addonsOutputs()
	if err != nil {
		return "", err
	}
	content, err := a.parser.ParseBackendService(template.ServiceOpts{
		Variables:   a.manifest.Variables,
		Secrets:     a.manifest.Secrets,
		NestedStack: outputs,
		HealthCheck: a.manifest.Image.HealthCheckOpts(),
	})
	if err != nil {
		return "", fmt.Errorf("parse backend app template: %w", err)
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (a *BackendApp) Parameters() []*cloudformation.Parameter {
	return append(a.app.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(BackendAppContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(a.manifest.Image.Port), 10)),
		},
	}...)
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (a *BackendApp) SerializedParameters() (string, error) {
	return a.app.templateConfiguration(a)
}
