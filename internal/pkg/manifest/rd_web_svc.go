// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	requestDrivenWebSvcManifestPath string = "workloads/services/rd-web/manifest.yml"
)

// RequestDrivenWebService holds the configuration to create a Request-Driven Web Service.
type RequestDrivenWebService struct {
	Workload                      `yaml:",inline"`
	RequestDrivenWebServiceConfig `yaml:",inline"`
	Environments                  map[string]*RequestDrivenWebServiceConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// RequestDrivenWebServiceConfig holds the configuration that can be overridden per environments.
type RequestDrivenWebServiceConfig struct {
	RequestDrivenWebServiceHttpConfig `yaml:"http,flow"`
	InstanceConfig                    AppRunnerInstanceConfig `yaml:",inline"`
	ImageConfig                       ImageWithPort           `yaml:"image"`
	Variables                         map[string]string       `yaml:"variables"`
	Tags                              map[string]string       `yaml:"tags"`
	Publish                           *PublishConfig          `yaml:"publish"`
}

type RequestDrivenWebServiceHttpConfig struct {
	HealthCheckConfiguration HealthCheckArgsOrString `yaml:"healthcheck"`
	Alias                    *string                 `yaml:"alias"`
}

// RequestDrivenWebServiceProps contains properties for creating a new request-driven web service manifest.
type RequestDrivenWebServiceProps struct {
	*WorkloadProps
	Port uint16
}

// AppRunnerInstanceConfig contains the instance configuration properties for an App Runner service.
type AppRunnerInstanceConfig struct {
	CPU      *int                 `yaml:"cpu"`
	Memory   *int                 `yaml:"memory"`
	Platform PlatformArgsOrString `yaml:"platform,omitempty"`
}

// NewRequestDrivenWebService creates a new Request-Driven Web Service manifest with default values.
func NewRequestDrivenWebService(props *RequestDrivenWebServiceProps) *RequestDrivenWebService {
	svc := newDefaultRequestDrivenWebService()
	svc.Name = aws.String(props.Name)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Port = aws.Uint16(props.Port)
	svc.parser = template.New()
	return svc
}

// newDefaultRequestDrivenWebService returns an empty RequestDrivenWebService with only the default values set.
func newDefaultRequestDrivenWebService() *RequestDrivenWebService {
	return &RequestDrivenWebService{
		Workload: Workload{
			Type: aws.String(RequestDrivenWebServiceType),
		},
		RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
			ImageConfig: ImageWithPort{},
			InstanceConfig: AppRunnerInstanceConfig{
				CPU:    aws.Int(1024),
				Memory: aws.Int(2048),
			},
		},
	}
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *RequestDrivenWebService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(requestDrivenWebSvcManifestPath, *s)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *RequestDrivenWebService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// BuildArgs returns a docker.BuildArguments object given a ws root directory.
func (s *RequestDrivenWebService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.BuildConfig(wsRoot)
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s RequestDrivenWebService) ApplyEnv(envName string) (WorkloadManifest, error) {
	overrideConfig, ok := s.Environments[envName]
	if !ok {
		return &s, nil
	}
	// Apply overrides to the original service configuration.
	err := mergo.Merge(&s, RequestDrivenWebService{
		RequestDrivenWebServiceConfig: *overrideConfig,
	}, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue, mergo.WithTransformers(workloadTransformer{}))

	if err != nil {
		return nil, err
	}
	s.Environments = nil
	return &s, nil
}
