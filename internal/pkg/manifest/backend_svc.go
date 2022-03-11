// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	backendSvcManifestPath = "workloads/services/backend/manifest.yml"
)

// BackendService holds the configuration to create a backend service manifest.
type BackendService struct {
	Workload             `yaml:",inline"`
	BackendServiceConfig `yaml:",inline"`
	// Use *BackendServiceConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*BackendServiceConfig `yaml:",flow"`

	parser template.Parser
}

// BackendServiceConfig holds the configuration that can be overridden per environments.
type BackendServiceConfig struct {
	ImageConfig      ImageWithHealthcheckAndOptionalPort `yaml:"image,flow"`
	ImageOverride    `yaml:",inline"`
	TaskConfig       `yaml:",inline"`
	Logging          Logging                   `yaml:"logging,flow"`
	Sidecars         map[string]*SidecarConfig `yaml:"sidecars"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
	Network          NetworkConfig             `yaml:"network"`
	PublishConfig    PublishConfig             `yaml:"publish"`
	TaskDefOverrides []OverrideRule            `yaml:"taskdef_overrides"`
}

// BackendServiceProps represents the configuration needed to create a backend service.
type BackendServiceProps struct {
	WorkloadProps
	Port        uint16
	HealthCheck ContainerHealthCheck // Optional healthcheck configuration.
	Platform    PlatformArgsOrString // Optional platform configuration.
}

// NewBackendService applies the props to a default backend service configuration with
// minimal task sizes, single replica, no healthcheck, and then returns it.
func NewBackendService(props BackendServiceProps) *BackendService {
	svc := newDefaultBackendService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.BackendServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.BackendServiceConfig.ImageConfig.Image.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.BackendServiceConfig.ImageConfig.Port = uint16P(props.Port)
	svc.BackendServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	svc.BackendServiceConfig.Platform = props.Platform
	if isWindowsPlatform(props.Platform) {
		svc.BackendServiceConfig.TaskConfig.CPU = aws.Int(MinWindowsTaskCPU)
		svc.BackendServiceConfig.TaskConfig.Memory = aws.Int(MinWindowsTaskMemory)
	}
	svc.parser = template.New()
	return svc
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *BackendService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(backendSvcManifestPath, *s, template.WithFuncs(map[string]interface{}{
		"fmtSlice":   template.FmtSliceFunc,
		"quoteSlice": template.QuoteSliceFunc,
	}))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// Port returns the exposed the exposed port in the manifest.
// If the backend service is not meant to be reachable, then ok is set to false.
func (s *BackendService) Port() (port uint16, ok bool) {
	value := s.BackendServiceConfig.ImageConfig.Port
	if value == nil {
		return 0, false
	}
	return aws.Uint16Value(value), true
}

// Publish returns the list of topics where notifications can be published.
func (s *BackendService) Publish() []Topic {
	return s.BackendServiceConfig.PublishConfig.Topics
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *BackendService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// BuildArgs returns a docker.BuildArguments object for the service given a workspace root directory.
func (s *BackendService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.Image.BuildConfig(wsRoot)
}

// EnvFile returns the location of the env file against the ws root directory.
func (s *BackendService) EnvFile() string {
	return aws.StringValue(s.TaskConfig.EnvFile)
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s BackendService) ApplyEnv(envName string) (WorkloadManifest, error) {
	overrideConfig, ok := s.Environments[envName]
	if !ok {
		return &s, nil
	}

	if overrideConfig == nil {
		return &s, nil
	}

	// Apply overrides to the original service s.
	for _, t := range defaultTransformers {
		err := mergo.Merge(&s, BackendService{
			BackendServiceConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))
		if err != nil {
			return nil, err
		}
	}
	s.Environments = nil
	return &s, nil
}

// newDefaultBackendService returns a backend service with minimal task sizes and a single replica.
func newDefaultBackendService() *BackendService {
	return &BackendService{
		Workload: Workload{
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: BackendServiceType,
					},
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementP(PublicSubnetPlacement),
				},
			},
		},
	}
}
