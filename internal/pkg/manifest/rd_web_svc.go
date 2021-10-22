// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"

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
	StartCommand                      *string                 `yaml:"command"`
	Tags                              map[string]string       `yaml:"tags"`
	PublishConfig                     PublishConfig           `yaml:"publish"`
}

type RequestDrivenWebServiceHttpConfig struct {
	HealthCheckConfiguration HealthCheckArgsOrString `yaml:"healthcheck"`
	Alias                    *string                 `yaml:"alias"`
}

// AppRunnerInstanceConfig contains the instance configuration properties for an App Runner service.
type AppRunnerInstanceConfig struct {
	CPU      *int                 `yaml:"cpu"`
	Memory   *int                 `yaml:"memory"`
	Platform PlatformArgsOrString `yaml:"platform,omitempty"`
}

// RequestDrivenWebServiceProps contains properties for creating a new request-driven web service manifest.
type RequestDrivenWebServiceProps struct {
	*WorkloadProps
	Port     uint16
	Platform PlatformArgsOrString
}

// NewRequestDrivenWebService creates a new Request-Driven Web Service manifest with default values.
func NewRequestDrivenWebService(props *RequestDrivenWebServiceProps) *RequestDrivenWebService {
	svc := newDefaultRequestDrivenWebService()
	svc.Name = aws.String(props.Name)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Image.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.RequestDrivenWebServiceConfig.ImageConfig.Port = aws.Uint16(props.Port)
	svc.RequestDrivenWebServiceConfig.InstanceConfig.Platform = props.Platform
	svc.parser = template.New()
	return svc
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

// Port returns the exposed the exposed port in the manifest.
// A RequestDrivenWebService always has a port exposed therefore the boolean is always true.
func (s *RequestDrivenWebService) Port() (port uint16, ok bool) {
	return aws.Uint16Value(s.ImageConfig.Port), true
}

// Publish returns the list of topics where notifications can be published.
func (s *RequestDrivenWebService) Publish() []Topic {
	return s.RequestDrivenWebServiceConfig.PublishConfig.Topics
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *RequestDrivenWebService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// TaskPlatform returns the platform for the service.
func (s *RequestDrivenWebService) TaskPlatform() (*string, error) {
	if s.InstanceConfig.Platform.IsEmpty() {
		return nil, nil
	}
	return aws.String(platformString(s.InstanceConfig.Platform.OS(), s.InstanceConfig.Platform.Arch())), nil
}

// BuildArgs returns a docker.BuildArguments object given a ws root directory.
func (s *RequestDrivenWebService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.Image.BuildConfig(wsRoot)
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s RequestDrivenWebService) ApplyEnv(envName string) (WorkloadManifest, error) {
	overrideConfig, ok := s.Environments[envName]
	if !ok {
		return &s, nil
	}
	// Apply overrides to the original service configuration.
	for _, t := range defaultTransformers {
		err := mergo.Merge(&s, RequestDrivenWebService{
			RequestDrivenWebServiceConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))
		if err != nil {
			return nil, err
		}
	}

	s.Environments = nil
	return &s, nil
}

// WindowsCompatibility disallows unsupported services when deploying Windows containers on Fargate.
// Here, this method is simply satisfying the WorkloadManifest interface.
func (s *RequestDrivenWebService) windowsCompatibility() error {
	if s.InstanceConfig.Platform.IsEmpty() {
		return nil
	}
	// Error out if user added Windows as platform in manifest.
	if isWindowsPlatform(s.InstanceConfig.Platform) {
		return errAppRunnerInvalidPlatformWindows
	}
	if s.InstanceConfig.Platform.Arch() != ArchAMD64 || s.InstanceConfig.Platform.Arch() != ArchX86 {
		return fmt.Errorf("App Runner services can only build on %s and %s architectures", ArchAMD64, ArchX86)
	}
	return nil
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
