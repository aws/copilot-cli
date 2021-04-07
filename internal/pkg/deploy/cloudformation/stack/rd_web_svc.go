// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type requestDrivenWebSvcReadParser interface {
	template.ReadParser
	ParseRequestDrivenWebService(template.WorkloadOpts) (*template.Content, error)
}

// RequestDrivenWebService represents the configuration needed to create a CloudFormation stack from a request-drive web service manifest.
type RequestDrivenWebService struct {
	*appRunnerWkld
	manifest *manifest.RequestDrivenWebService

	parser requestDrivenWebSvcReadParser
}

// NewRequestDrivenWebService creates a new RequestDrivenWebService stack from a manifest file.
func NewRequestDrivenWebService(mft *manifest.RequestDrivenWebService, env, app string, rc RuntimeConfig) (*RequestDrivenWebService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest, err := mft.ApplyEnv(env) // Apply environment overrides to the manifest values.
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", env, err)
	}
	return &RequestDrivenWebService{
		appRunnerWkld: &appRunnerWkld{
			wkld: &wkld{
				name:   aws.StringValue(mft.Name),
				env:    env,
				app:    app,
				rc:     rc,
				image:  envManifest.ImageConfig,
				addons: addons,
				parser: parser,
			},
			instanceConfig:    envManifest.InstanceConfig,
			imageConfig:       envManifest.ImageConfig,
			healthCheckConfig: envManifest.HealthCheckConfiguration,
		},
		manifest: envManifest,
		parser:   parser,
	}, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *RequestDrivenWebService) Template() (string, error) {
	content, err := s.parser.ParseRequestDrivenWebService(template.WorkloadOpts{
		Variables: s.manifest.Variables,
		Tags:      s.manifest.Tags,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *RequestDrivenWebService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
