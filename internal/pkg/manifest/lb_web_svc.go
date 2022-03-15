// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	lbWebSvcManifestPath = "workloads/services/lb-web/manifest.yml"
)

// Default values for HTTPHealthCheck for a load balanced web service.
const (
	DefaultHealthCheckPath        = "/"
	DefaultHealthCheckGracePeriod = 60
)

const (
	GRPCProtocol = "gRPC" // GRPCProtocol is the HTTP protocol version for gRPC.
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
	ImageConfig      ImageWithPortAndHealthcheck `yaml:"image,flow"`
	ImageOverride    `yaml:",inline"`
	RoutingRule      RoutingRuleConfigOrBool `yaml:"http,flow"`
	TaskConfig       `yaml:",inline"`
	Logging          `yaml:"logging,flow"`
	Sidecars         map[string]*SidecarConfig        `yaml:"sidecars"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
	Network          NetworkConfig                    `yaml:"network"`
	PublishConfig    PublishConfig                    `yaml:"publish"`
	TaskDefOverrides []OverrideRule                   `yaml:"taskdef_overrides"`
	NLBConfig        NetworkLoadBalancerConfiguration `yaml:"nlb"`
}

// LoadBalancedWebServiceProps contains properties for creating a new load balanced fargate service manifest.
type LoadBalancedWebServiceProps struct {
	*WorkloadProps
	Path string
	Port uint16

	HTTPVersion string               // Optional http protocol version such as gRPC, HTTP2.
	HealthCheck ContainerHealthCheck // Optional healthcheck configuration.
	Platform    PlatformArgsOrString // Optional platform configuration.
}

// NewLoadBalancedWebService creates a new public load balanced web service, receives all the requests from the load balancer,
// has a single task with minimal CPU and memory thresholds, and sets the default health check path to "/".
func NewLoadBalancedWebService(props *LoadBalancedWebServiceProps) *LoadBalancedWebService {
	svc := newDefaultHTTPLoadBalancedWebService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Image.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.LoadBalancedWebServiceConfig.ImageConfig.Port = aws.Uint16(props.Port)
	svc.LoadBalancedWebServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	svc.LoadBalancedWebServiceConfig.Platform = props.Platform
	if isWindowsPlatform(props.Platform) {
		svc.LoadBalancedWebServiceConfig.TaskConfig.CPU = aws.Int(MinWindowsTaskCPU)
		svc.LoadBalancedWebServiceConfig.TaskConfig.Memory = aws.Int(MinWindowsTaskMemory)
	}
	if props.HTTPVersion != "" {
		svc.RoutingRule.ProtocolVersion = &props.HTTPVersion
	}
	svc.RoutingRule.Path = aws.String(props.Path)
	svc.parser = template.New()
	return svc
}

// newDefaultHTTPLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set, including default HTTP configurations.
func newDefaultHTTPLoadBalancedWebService() *LoadBalancedWebService {
	lbws := newDefaultLoadBalancedWebService()
	lbws.RoutingRule = RoutingRuleConfigOrBool{
		RoutingRuleConfiguration: RoutingRuleConfiguration{
			HealthCheck: HealthCheckArgsOrString{
				HealthCheckPath: aws.String(DefaultHealthCheckPath),
			},
		},
	}
	return lbws
}

// newDefaultLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set, without any load balancer configuration.
func newDefaultLoadBalancedWebService() *LoadBalancedWebService {
	return &LoadBalancedWebService{
		Workload: Workload{
			Type: aws.String(LoadBalancedWebServiceType),
		},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: LoadBalancedWebServiceType,
					},
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: placementP(PublicSubnetPlacement),
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

// Port returns the exposed port in the manifest.
// A LoadBalancedWebService always has a port exposed therefore the boolean is always true.
func (s *LoadBalancedWebService) Port() (port uint16, ok bool) {
	return aws.Uint16Value(s.ImageConfig.Port), true
}

// Publish returns the list of topics where notifications can be published.
func (s *LoadBalancedWebService) Publish() []Topic {
	return s.LoadBalancedWebServiceConfig.PublishConfig.Topics
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *LoadBalancedWebService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// BuildArgs returns a docker.BuildArguments object given a ws root directory.
func (s *LoadBalancedWebService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.Image.BuildConfig(wsRoot)
}

// EnvFile returns the location of the env file against the ws root directory.
func (s *LoadBalancedWebService) EnvFile() string {
	return aws.StringValue(s.TaskConfig.EnvFile)
}

// HasAliases returns true if the Load-Balanced Web Service uses aliases, either for ALB or NLB.
func (s *LoadBalancedWebService) HasAliases() bool {
	return !s.RoutingRule.Alias.IsEmpty() || !s.NLBConfig.Aliases.IsEmpty()
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

	for _, t := range defaultTransformers {
		// Apply overrides to the original service s.
		err := mergo.Merge(&s, LoadBalancedWebService{
			LoadBalancedWebServiceConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))

		if err != nil {
			return nil, err
		}
	}

	s.Environments = nil
	return &s, nil
}

// RoutingRuleConfigOrBool holds advanced configuration for routing rule or a boolean switch.
type RoutingRuleConfigOrBool struct {
	RoutingRuleConfiguration
	Enabled *bool
}

// Disabled returns true if the routing rule configuration is explicitly disabled.
func (r *RoutingRuleConfigOrBool) Disabled() bool {
	return r.Enabled != nil && !aws.BoolValue(r.Enabled)
}

// UnmarshalYAML implements the yaml(v3) interface. It allows https routing rule to be specified as a
// bool or a struct alternately.
func (r *RoutingRuleConfigOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.RoutingRuleConfiguration); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.RoutingRuleConfiguration.isEmpty() {
		// Unmarshalled successfully to r.RoutingRuleConfiguration, unset r.Enabled, and return.
		r.Enabled = nil
		return nil
	}

	if err := value.Decode(&r.Enabled); err != nil {
		return errors.New(`cannot marshal "http" field into bool or map`)
	}
	return nil
}

// RoutingRuleConfiguration holds the path to route requests to the service.
type RoutingRuleConfiguration struct {
	Path                *string                 `yaml:"path"`
	ProtocolVersion     *string                 `yaml:"version"`
	HealthCheck         HealthCheckArgsOrString `yaml:"healthcheck"`
	Stickiness          *bool                   `yaml:"stickiness"`
	Alias               Alias                   `yaml:"alias"`
	DeregistrationDelay *time.Duration          `yaml:"deregistration_delay"`
	// TargetContainer is the container load balancer routes traffic to.
	TargetContainer          *string `yaml:"target_container"`
	TargetContainerCamelCase *string `yaml:"targetContainer"` // "targetContainerCamelCase" for backwards compatibility
	AllowedSourceIps         []IPNet `yaml:"allowed_source_ips"`
}

func (r *RoutingRuleConfiguration) targetContainer() *string {
	if r.TargetContainer == nil && r.TargetContainerCamelCase == nil {
		return nil
	}
	if r.TargetContainer != nil {
		return r.TargetContainer
	}
	return r.TargetContainerCamelCase
}

func (r *RoutingRuleConfiguration) isEmpty() bool {
	return r.Path == nil && r.ProtocolVersion == nil && r.HealthCheck.IsEmpty() && r.Stickiness == nil && r.Alias.IsEmpty() &&
		r.DeregistrationDelay == nil && r.TargetContainer == nil && r.TargetContainerCamelCase == nil && r.AllowedSourceIps == nil
}

// NetworkLoadBalancerConfiguration holds options for a network load balancer
type NetworkLoadBalancerConfiguration struct {
	Port            *string            `yaml:"port"`
	HealthCheck     NLBHealthCheckArgs `yaml:"healthcheck"`
	TargetContainer *string            `yaml:"target_container"`
	TargetPort      *int               `yaml:"target_port"`
	SSLPolicy       *string            `yaml:"ssl_policy"`
	Stickiness      *bool              `yaml:"stickiness"`
	Aliases         Alias              `yaml:"alias"`
}

func (c *NetworkLoadBalancerConfiguration) IsEmpty() bool {
	return c.Port == nil && c.HealthCheck.isEmpty() && c.TargetContainer == nil && c.TargetPort == nil &&
		c.SSLPolicy == nil && c.Stickiness == nil && c.Aliases.IsEmpty()
}

// IPNet represents an IP network string. For example: 10.1.0.0/16
type IPNet string

// Alias is a custom type which supports unmarshaling "http.alias" yaml which
// can either be of type string or type slice of string.
type Alias stringSliceOrString

// IsEmpty returns empty if Alias is empty.
func (e *Alias) IsEmpty() bool {
	return e.String == nil && e.StringSlice == nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Alias
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (e *Alias) UnmarshalYAML(value *yaml.Node) error {
	if err := unmarshalYAMLToStringSliceOrString((*stringSliceOrString)(e), value); err != nil {
		return errUnmarshalAlias
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

// ToString converts an Alias to a string.
func (e *Alias) ToString() string {
	if e.String != nil {
		return aws.StringValue(e.String)
	}
	return strings.Join(e.StringSlice, ",")
}
