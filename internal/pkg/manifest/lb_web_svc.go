// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	lbWebSvcManifestPath = "workloads/services/lb-web/manifest.yml"
)

// Default values for HTTPHealthCheck for a load balanced web service.
const (
	DefaultHealthCheckPath        = "/"
	DefaultHealthCheckGracePeriod = 60
)

var (
	errUnmarshalHealthCheckArgs = errors.New("can't unmarshal healthcheck field into string or compose-style map")
)

// durationp is a utility function used to convert a time.Duration to a pointer. Useful for YAML unmarshaling
// and template execution.
func durationp(v time.Duration) *time.Duration {
	return &v
}

// LoadBalancedWebService holds the configuration to build a container image with an exposed port that receives
// requests through a load balancer with AWS Fargate as the compute engine.
type LoadBalancedWebService struct {
	Workload                     `yaml:",inline"`
	LoadBalancedWebServiceConfig `yaml:",inline"`
	// Use *LoadBalancedWebServiceConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*LoadBalancedWebServiceConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// LoadBalancedWebServiceConfig holds the configuration for a load balanced web service.
type LoadBalancedWebServiceConfig struct {
	ImageConfig   ImageWithPortAndHealthcheck `yaml:"image,flow"`
	ImageOverride `yaml:",inline"`
	RoutingRule   `yaml:"http,flow"`
	TaskConfig    `yaml:",inline"`
	*Logging      `yaml:"logging,flow"`
	Sidecars      map[string]*SidecarConfig `yaml:"sidecars"`
	Network       *NetworkConfig            `yaml:"network"` // TODO: the type needs to be updated after we upgrade mergo
	Publish       *PublishConfig            `yaml:"publish"`
}

// RoutingRule holds the path to route requests to the service.
type RoutingRule struct {
	Path                *string                 `yaml:"path"`
	HealthCheck         HealthCheckArgsOrString `yaml:"healthcheck"`
	Stickiness          *bool                   `yaml:"stickiness"`
	Alias               *Alias                  `yaml:"alias"`
	DeregistrationDelay *time.Duration          `yaml:"deregistration_delay"`
	// TargetContainer is the container load balancer routes traffic to.
	TargetContainer          *string   `yaml:"target_container"`
	TargetContainerCamelCase *string   `yaml:"targetContainer"`    // "targetContainerCamelCase" for backwards compatibility
	AllowedSourceIps         *[]string `yaml:"allowed_source_ips"` // TODO: the type needs to be updated after we upgrade mergo
}

// LoadBalancedWebServiceProps contains properties for creating a new load balanced fargate service manifest.
type LoadBalancedWebServiceProps struct {
	*WorkloadProps
	Path        string
	Port        uint16
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
	Platform    *PlatformArgsOrString // Optional platform configuration.
}

// Alias is a custom type which supports unmarshaling "http.alias" yaml which
// can either be of type string or type slice of string.
type Alias stringSliceOrString

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Alias
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (e *Alias) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshalYAMLToStringSliceOrString((*stringSliceOrString)(e), unmarshal); err != nil {
		return errUnmarshalEntryPoint
	}
	return nil
}

// ToStringSlice converts an Alias to a slice of string using shell-style rules.
func (e *Alias) ToStringSlice() ([]string, error) {
	out, err := toStringSlice((*stringSliceOrString)(e))
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NewLoadBalancedWebService creates a new public load balanced web service, receives all the requests from the load balancer,
// has a single task with minimal CPU and memory thresholds, and sets the default health check path to "/".
func NewLoadBalancedWebService(props *LoadBalancedWebServiceProps) *LoadBalancedWebService {
	svc := newDefaultLoadBalancedWebService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Port = aws.Uint16(props.Port)
	svc.LoadBalancedWebServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	if props.Platform != nil {
		svc.LoadBalancedWebServiceConfig.Platform = props.Platform
		if isWindowsPlatform(props.Platform) {
<<<<<<< HEAD
			svc.LoadBalancedWebServiceConfig.TaskConfig.CPU = aws.Int(1024)
			svc.LoadBalancedWebServiceConfig.TaskConfig.Memory = aws.Int(2048)
=======
			svc.LoadBalancedWebServiceConfig.TaskConfig.CPU = aws.Int(windowsTaskCPU)
			svc.LoadBalancedWebServiceConfig.TaskConfig.Memory = aws.Int(windowsTaskMemory)
>>>>>>> archer/feat/pencere
		}
	}
	svc.RoutingRule.Path = aws.String(props.Path)
	svc.parser = template.New()
	return svc
}

// newDefaultLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set.
func newDefaultLoadBalancedWebService() *LoadBalancedWebService {
	return &LoadBalancedWebService{
		Workload: Workload{
			Type: aws.String(LoadBalancedWebServiceType),
		},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{},
			RoutingRule: RoutingRule{
				HealthCheck: HealthCheckArgsOrString{
					HealthCheckPath: aws.String(DefaultHealthCheckPath),
				},
			},
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

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *LoadBalancedWebService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(lbWebSvcManifestPath, *s)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *LoadBalancedWebService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// TaskPlatform returns the platform for the service.
func (t *TaskConfig) TaskPlatform() (*string, error) {
	if t.Platform == nil {
		return nil, nil
	}
	return aws.String(platformString(t.Platform.OS(), t.Platform.Arch())), nil
}

// IsWindows returns whether or not the service is building with a Windows OS.
func (t TaskConfig) IsWindows() bool {
	if t.Platform == nil {
		return false
	}
	return isWindowsPlatform(t.Platform)
}

// BuildArgs returns a docker.BuildArguments object given a ws root directory.
func (s *LoadBalancedWebService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.BuildConfig(wsRoot)
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s LoadBalancedWebService) ApplyEnv(envName string) (WorkloadManifest, error) {
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
	err := mergo.Merge(&s, LoadBalancedWebService{
		LoadBalancedWebServiceConfig: *overrideConfig,
	}, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue, mergo.WithTransformers(workloadTransformer{}))

	if err != nil {
		return nil, err
	}
	s.Environments = nil
	return &s, nil
}

// windowsCompatibility disallows unsupported services when deploying Windows containers on Fargate.
func (s *LoadBalancedWebService) windowsCompatibility() error {
	if !s.IsWindows() {
		return nil
	}
	// Exec is not supported.
	if aws.BoolValue(s.ExecuteCommand.Enable) {
		return errors.New(`'exec' is not supported when deploying a Windows container`)
	}
	// EFS is not supported.
	if s.Storage != nil {
		for _, volume := range s.Storage.Volumes {
			if !volume.EmptyVolume() {
				return errors.New(`'EFS' is not supported when deploying a Windows container`)
			}
		}
	}
	return nil
}
