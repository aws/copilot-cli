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

// BackendServiceProps represents the configuration needed to create a backend service.
type BackendServiceProps struct {
	WorkloadProps
	Port        uint16
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
}

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
	ImageConfig   ImageWithPortAndHealthcheck `yaml:"image,flow"`
	ImageOverride `yaml:",inline"`
	TaskConfig    `yaml:",inline"`
	*Logging      `yaml:"logging,flow"`
	Sidecars      map[string]*SidecarConfig `yaml:"sidecars"`
	Network       *NetworkConfig            `yaml:"network"`
}

// NewBackendService applies the props to a default backend service configuration with
// minimal task sizes, single replica, no healthcheck, and then returns it.
func NewBackendService(props BackendServiceProps) *BackendService {
	svc := newDefaultBackendService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.BackendServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.BackendServiceConfig.ImageConfig.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.BackendServiceConfig.ImageConfig.Port = uint16P(props.Port)
	svc.BackendServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	svc.parser = template.New()
	return svc
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *BackendService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(backendSvcManifestPath, *s, template.WithFuncs(map[string]interface{}{
		"fmtSlice":   template.FmtSliceFunc,
		"quoteSlice": template.QuoteSliceFunc,
		"dirName":    tplDirName,
	}))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *BackendService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// BuildArgs returns a docker.BuildArguments object for the service given a workspace root directory
func (s *BackendService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.BuildConfig(wsRoot)
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

	envCount := overrideConfig.TaskConfig.Count
	if !envCount.IsEmpty() {
		s.TaskConfig.Count = envCount
	}

	// Apply overrides to the original service s.
	err := mergo.Merge(&s, BackendService{
		BackendServiceConfig: *overrideConfig,
	}, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue, mergo.WithTransformers(workloadTransformer{}))

	if err != nil {
		return nil, err
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
			ImageConfig: ImageWithPortAndHealthcheck{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: &NetworkConfig{
				VPC: &vpcConfig{
					Placement: stringP(PublicSubnetPlacement),
				},
			},
		},
	}
}
