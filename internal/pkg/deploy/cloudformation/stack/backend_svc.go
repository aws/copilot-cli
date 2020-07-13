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
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest, err := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", env, err)
	}
	return &BackendService{
		svc: &svc{
			name:   aws.StringValue(mft.Name),
			env:    env,
			app:    app,
			tc:     envManifest.BackendServiceConfig.TaskConfig,
			rc:     rc,
			parser: parser,
			addons: addons,
		},
		manifest: envManifest,

		parser: parser,
	}, nil
}

// Template returns the CloudFormation template for the backend service.
func (s *BackendService) Template() (string, error) {
	outputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	sidecars, err := s.manifest.Sidecar.SidecarsOpts()
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for service %s: %w", s.name, err)
	}
	content, err := s.parser.ParseBackendService(template.ServiceOpts{
		Variables:   s.manifest.BackendServiceConfig.Variables,
		Secrets:     s.manifest.BackendServiceConfig.Secrets,
		NestedStack: outputs,
		Sidecars:    sidecars,
		HealthCheck: s.manifest.BackendServiceConfig.Image.HealthCheckOpts(),
		LogConfig:   s.manifest.LogConfigOpts(),
	})
	if err != nil {
		return "", fmt.Errorf("parse backend service template: %w", err)
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *BackendService) Parameters() ([]*cloudformation.Parameter, error) {
	return append(s.svc.Parameters(), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(BackendServiceContainerPortParamKey),
			ParameterValue: aws.String(strconv.FormatUint(uint64(aws.Uint16Value(s.manifest.BackendServiceConfig.Image.Port)), 10)),
		},
	}...), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *BackendService) SerializedParameters() (string, error) {
	return s.svc.templateConfiguration(s)
}
