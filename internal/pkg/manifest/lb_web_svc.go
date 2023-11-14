// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"maps"
	"strconv"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

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
	DefaultHealthCheckAdminPath   = "admin"
	DefaultHealthCheckGracePeriod = 60
	DefaultDeregistrationDelay    = 60
)

const (
	GRPCProtocol   = "gRPC" // GRPCProtocol is the HTTP protocol version for gRPC.
	commonGRPCPort = uint16(50051)
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
	HTTPOrBool       HTTPOrBool `yaml:"http,flow"`
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

	if props.Port == commonGRPCPort {
		log.Infof("Detected port %s, setting HTTP protocol version to %s in the manifest.\n",
			color.HighlightUserInput(strconv.Itoa(int(props.Port))), color.HighlightCode(GRPCProtocol))
		svc.HTTPOrBool.Main.ProtocolVersion = aws.String(GRPCProtocol)
	}
	svc.HTTPOrBool.Main.Path = aws.String(props.Path)
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
	lbws.HTTPOrBool = HTTPOrBool{
		HTTP: HTTP{
			Main: RoutingRule{
				HealthCheck: HealthCheckArgsOrString{
					Union: BasicToUnion[string, HTTPHealthCheckArgs](DefaultHealthCheckPath),
				},
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
	if !s.HTTPOrBool.Disabled() {
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

// ContainerDependencies returns a map of ContainerDependency objects for the LoadBalancedWebService
// including dependencies for its main container, any logging sidecar, and additional sidecars.
func (s *LoadBalancedWebService) ContainerDependencies() map[string]ContainerDependency {
	return containerDependencies(aws.StringValue(s.Name), s.ImageConfig.Image, s.Logging, s.Sidecars)
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

// NetworkLoadBalancerConfiguration holds options for a network load balancer.
type NetworkLoadBalancerConfiguration struct {
	Listener            NetworkLoadBalancerListener   `yaml:",inline"`
	Aliases             Alias                         `yaml:"alias"`
	AdditionalListeners []NetworkLoadBalancerListener `yaml:"additional_listeners"`
}

// NetworkLoadBalancerListener holds listener configuration for NLB.
type NetworkLoadBalancerListener struct {
	Port                *string            `yaml:"port"`
	HealthCheck         NLBHealthCheckArgs `yaml:"healthcheck"`
	TargetContainer     *string            `yaml:"target_container"`
	TargetPort          *int               `yaml:"target_port"`
	SSLPolicy           *string            `yaml:"ssl_policy"`
	Stickiness          *bool              `yaml:"stickiness"`
	DeregistrationDelay *time.Duration     `yaml:"deregistration_delay"`
}

// IsEmpty returns true if NetworkLoadBalancerConfiguration is empty.
func (c *NetworkLoadBalancerConfiguration) IsEmpty() bool {
	return c.Aliases.IsEmpty() && c.Listener.IsEmpty() && len(c.AdditionalListeners) == 0
}

// IsEmpty returns true if NetworkLoadBalancerListener is empty.
func (c *NetworkLoadBalancerListener) IsEmpty() bool {
	return c.Port == nil && c.HealthCheck.isEmpty() && c.TargetContainer == nil && c.TargetPort == nil &&
		c.SSLPolicy == nil && c.Stickiness == nil && c.DeregistrationDelay == nil
}

// HealthCheckPort returns the port a HealthCheck is set to for a NetworkLoadBalancerListener.
func (listener NetworkLoadBalancerListener) HealthCheckPort(mainContainerPort *uint16) (uint16, error) {
	// healthCheckPort is defined by Listener.HealthCheck.Port, with fallback on Listener.TargetPort, then Listener.Port.
	if listener.HealthCheck.Port != nil {
		return uint16(aws.IntValue(listener.HealthCheck.Port)), nil
	}
	if listener.TargetPort != nil {
		return uint16(aws.IntValue(listener.TargetPort)), nil
	}
	if listener.Port != nil {
		port, _, err := ParsePortMapping(listener.Port)
		if err != nil {
			return 0, err
		}
		parsedPort, err := strconv.ParseUint(aws.StringValue(port), 10, 16)
		if err != nil {
			return 0, err
		}
		return uint16(parsedPort), nil
	}
	if mainContainerPort != nil {
		return aws.Uint16Value(mainContainerPort), nil
	}
	return 0, nil
}

// ExposedPorts returns all the ports that are container ports available to receive traffic.
func (lbws *LoadBalancedWebService) ExposedPorts() (ExposedPortsIndex, error) {
	exposedPorts := make(map[uint16]ExposedPort)
	workloadName := aws.StringValue(lbws.Name)
	// port from sidecar[x].image.port.
	for name, sidecar := range lbws.Sidecars {
		newExposedPorts, err := sidecar.exposePorts(exposedPorts, name)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		maps.Copy(exposedPorts, newExposedPorts)
	}
	// port from http.target_port and http.additional_rules[x].target_port
	for _, rule := range lbws.HTTPOrBool.RoutingRules() {
		maps.Copy(exposedPorts, rule.exposePorts(exposedPorts, workloadName))
	}

	// port from nlb.target_port and nlb.additional_listeners[x].target_port
	for _, listener := range lbws.NLBConfig.NLBListeners() {
		newExposedPorts, err := listener.exposePorts(exposedPorts, workloadName)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		maps.Copy(exposedPorts, newExposedPorts)
	}
	// port from image.port
	maps.Copy(exposedPorts, lbws.ImageConfig.exposePorts(exposedPorts, workloadName))

	portsForContainer, containerForPort := prepareParsedExposedPortsMap(exposedPorts)
	return ExposedPortsIndex{
		WorkloadName:      workloadName,
		PortsForContainer: portsForContainer,
		ContainerForPort:  containerForPort,
	}, nil
}

// NLBListeners returns main as well as additional listeners as a list of NetworkLoadBalancerListener.
func (cfg NetworkLoadBalancerConfiguration) NLBListeners() []NetworkLoadBalancerListener {
	if cfg.IsEmpty() {
		return nil
	}
	return append([]NetworkLoadBalancerListener{cfg.Listener}, cfg.AdditionalListeners...)
}
