// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/imdario/mergo"

	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
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
	parser       template.Parser
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
	DeployConfig     DeploymentConfig                 `yaml:"deployment"`
	Observability    Observability                    `yaml:"observability"`
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
	for _, envName := range props.PrivateOnlyEnvironments {
		svc.Environments[envName] = &LoadBalancedWebServiceConfig{
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PrivateSubnetPlacement),
					},
				},
			},
		}
	}
	return svc
}

// newDefaultHTTPLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set, including default HTTP configurations.
func newDefaultHTTPLoadBalancedWebService() *LoadBalancedWebService {
	lbws := newDefaultLoadBalancedWebService()
	lbws.RoutingRule = RoutingRuleConfigOrBool{
		RoutingRuleConfiguration: RoutingRuleConfiguration{
			HealthCheck: HealthCheckArgsOrString{
				Union: BasicToUnion[string, HTTPHealthCheckArgs](DefaultHealthCheckPath),
			},
		},
	}
	return lbws
}

// newDefaultLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set, without any load balancer configuration.
func newDefaultLoadBalancedWebService() *LoadBalancedWebService {
	return &LoadBalancedWebService{
		Workload: Workload{
			Type: aws.String(manifestinfo.LoadBalancedWebServiceType),
		},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: manifestinfo.LoadBalancedWebServiceType,
					},
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PublicSubnetPlacement),
					},
				},
			},
		},
		Environments: map[string]*LoadBalancedWebServiceConfig{},
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

func (s *LoadBalancedWebService) requiredEnvironmentFeatures() []string {
	var features []string
	if !s.RoutingRule.Disabled() {
		features = append(features, template.ALBFeatureName)
	}
	features = append(features, s.Network.requiredEnvFeatures()...)
	features = append(features, s.Storage.requiredEnvFeatures()...)
	return features
}

// Port returns the exposed port in the manifest.
// A LoadBalancedWebService always has a port exposed therefore the boolean is always true.
func (s *LoadBalancedWebService) Port() (port uint16, ok bool) {
	return aws.Uint16Value(s.ImageConfig.Port), true
}

// Publish returns the list of topics where notifications can be published.
func (s *LoadBalancedWebService) Publish() []Topic {
	return s.LoadBalancedWebServiceConfig.PublishConfig.publishedTopics()
}

// BuildArgs returns a docker.BuildArguments object given a context directory.
func (s *LoadBalancedWebService) BuildArgs(contextDir string) (map[string]*DockerBuildArgs, error) {
	required, err := requiresBuild(s.ImageConfig.Image)
	if err != nil {
		return nil, err
	}
	// Creating an map to store buildArgs of all sidecar images and main container image.
	buildArgsPerContainer := make(map[string]*DockerBuildArgs, len(s.Sidecars)+1)
	if required {
		buildArgsPerContainer[aws.StringValue(s.Name)] = s.ImageConfig.Image.BuildConfig(contextDir)
	}
	return buildArgs(contextDir, buildArgsPerContainer, s.Sidecars)
}

// EnvFiles returns the locations of all env files against the ws root directory.
// This method returns a map[string]string where the keys are container names
// and the values are either env file paths or empty strings.
func (s *LoadBalancedWebService) EnvFiles() map[string]string {
	return envFiles(s.Name, s.TaskConfig, s.Logging, s.Sidecars)
}

func (s *LoadBalancedWebService) subnets() *SubnetListOrArgs {
	return &s.Network.VPC.Placement.Subnets
}

func (s LoadBalancedWebService) applyEnv(envName string) (workloadManifest, error) {
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

// NetworkLoadBalancerConfiguration holds options for a network load balancer
type NetworkLoadBalancerConfiguration struct {
	PrimaryRoutingRule     NetworkLoadBalancerRoutingRule   `yaml:",inline"`
	AdditionalRoutingRules []NetworkLoadBalancerRoutingRule // `yaml:"additional_rules"`
}

type NetworkLoadBalancerRoutingRule struct {
	Port            *string            `yaml:"port"`
	HealthCheck     NLBHealthCheckArgs `yaml:"healthcheck"`
	TargetContainer *string            `yaml:"target_container"`
	TargetPort      *int               `yaml:"target_port"`
	SSLPolicy       *string            `yaml:"ssl_policy"`
	Stickiness      *bool              `yaml:"stickiness"`
	Aliases         Alias              `yaml:"alias"`
}

func (c *NetworkLoadBalancerRoutingRule) IsEmpty() bool {
	return c.Port == nil && c.HealthCheck.isEmpty() && c.TargetContainer == nil && c.TargetPort == nil &&
		c.SSLPolicy == nil && c.Stickiness == nil && c.Aliases.IsEmpty()
}

// ExposedPorts returns all the ports that are container ports available to receive traffic.
func (lbws *LoadBalancedWebService) ExposedPorts() (ExposedPortsIndex, error) {
	var exposedPorts []ExposedPort
	workloadName := aws.StringValue(lbws.Name)
	// port from image.port.
	exposedPorts = append(exposedPorts, lbws.ImageConfig.exposedPorts(workloadName)...)
	// port from sidecar[x].image.port.
	for name, sidecar := range lbws.Sidecars {
		out, err := sidecar.exposedPorts(name)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		exposedPorts = append(exposedPorts, out...)
	}
	// port from http.target_port.
	exposedPorts = append(exposedPorts, lbws.RoutingRule.exposedPorts(exposedPorts, workloadName)...)
	// port from nlb.target_port.
	out, err := lbws.NLBConfig.PrimaryRoutingRule.exposedPorts(exposedPorts, workloadName)
	if err != nil {
		return ExposedPortsIndex{}, err
	}
	exposedPorts = append(exposedPorts, out...)
	// port from nlb.additional_rule[x].target_port.
	for _, additionalRule := range lbws.NLBConfig.AdditionalRoutingRules {
		out, err = additionalRule.exposedPorts(exposedPorts, workloadName)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		exposedPorts = append(exposedPorts, out...)
	}
	portsForContainer, containerForPort := prepareParsedExposedPortsMap(sortExposedPorts(exposedPorts))
	return ExposedPortsIndex{
		PortsForContainer: portsForContainer,
		ContainerForPort:  containerForPort,
	}, nil
}
