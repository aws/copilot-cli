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

// Parameter logical IDs for a backend service.
const (
	BackendServiceContainerPortParamKey = "ContainerPort"
)

type backendSvcReadParser interface {
	template.ReadParser
	ParseBackendService(template.ServiceOpts) (*template.Content, error)
}

// BackendService represents the configuration needed to create a CloudFormation stack from a backend service manifest.
type BackendService struct {
	*svc
	manifest *manifest.BackendService

	parser backendSvcReadParser
}

// NewBackendService creates a new BackendService stack from a manifest file.
func NewBackendService(mft *manifest.BackendService, env, app string, rc RuntimeConfig) (*BackendService, error) {
	parser := template.New()
	addons, err := addons.New(mft.Name)
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	return &BackendService{
		svc: &svc{
			name:   mft.Name,
			env:    env,
			app:    app,
			tc:     envManifest.TaskConfig,
			rc:     rc,
			parser: parser,
			addons: addons,
		},
		manifest: envManifest,

		parser: parser,
	}, nil
}

// Template returns the CloudFormation template for the backend service.
func (a *BackendService) Template() (string, error) {
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
		return "", fmt.Errorf("parse backend service template: %w", err)
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (a *BackendService) Parameters() []*cloudformation.Parameter {
	return append(a.svc.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(BackendServiceContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(a.manifest.Image.Port), 10)),
		},
	}...)
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (a *BackendService) SerializedParameters() (string, error) {
	return a.svc.templateConfiguration(a)
}
