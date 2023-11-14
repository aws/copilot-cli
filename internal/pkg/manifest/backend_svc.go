// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"maps"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
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
	parser       template.Parser
}

// BackendServiceConfig holds the configuration that can be overridden per environments.
type BackendServiceConfig struct {
	ImageConfig      ImageWithHealthcheckAndOptionalPort `yaml:"image,flow"`
	ImageOverride    `yaml:",inline"`
	HTTP             HTTP `yaml:"http,flow"`
	TaskConfig       `yaml:",inline"`
	Logging          Logging                   `yaml:"logging,flow"`
	Sidecars         map[string]*SidecarConfig `yaml:"sidecars"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
	Network          NetworkConfig             `yaml:"network"`
	PublishConfig    PublishConfig             `yaml:"publish"`
	TaskDefOverrides []OverrideRule            `yaml:"taskdef_overrides"`
	DeployConfig     DeploymentConfig          `yaml:"deployment"`
	Observability    Observability             `yaml:"observability"`
}

// BackendServiceProps represents the configuration needed to create a backend service.
type BackendServiceProps struct {
	WorkloadProps
	Port        uint16
	Path        string               // Optional path if multiple ports are exposed.
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
	for _, envName := range props.PrivateOnlyEnvironments {
		svc.Environments[envName] = &BackendServiceConfig{
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

func (s *BackendService) requiredEnvironmentFeatures() []string {
	var features []string
	if !s.HTTP.IsEmpty() {
		features = append(features, template.InternalALBFeatureName)
	}
	features = append(features, s.Network.requiredEnvFeatures()...)
	features = append(features, s.Storage.requiredEnvFeatures()...)
	return features
}

// Port returns the exposed port in the manifest.
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
	return s.BackendServiceConfig.PublishConfig.publishedTopics()
}

// BuildArgs returns a docker.BuildArguments object for the service given a context directory.
func (s *BackendService) BuildArgs(contextDir string) (map[string]*DockerBuildArgs, error) {
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
func (s *BackendService) EnvFiles() map[string]string {
	return envFiles(s.Name, s.TaskConfig, s.Logging, s.Sidecars)
}

// ContainerDependencies returns a map of ContainerDependency objects for the BackendService
// including dependencies for its main container, any logging sidecar, and additional sidecars.
func (s *BackendService) ContainerDependencies() map[string]ContainerDependency {
	return containerDependencies(aws.StringValue(s.Name), s.ImageConfig.Image, s.Logging, s.Sidecars)
}

func (s *BackendService) subnets() *SubnetListOrArgs {
	return &s.Network.VPC.Placement.Subnets
}

func (s BackendService) applyEnv(envName string) (workloadManifest, error) {
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
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: manifestinfo.BackendServiceType,
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
		Environments: map[string]*BackendServiceConfig{},
	}
}

// ExposedPorts returns all the ports that are container ports available to receive traffic.
func (b *BackendService) ExposedPorts() (ExposedPortsIndex, error) {
	exposedPorts := make(map[uint16]ExposedPort)

	workloadName := aws.StringValue(b.Name)
	for name, sidecar := range b.Sidecars {
		newExposedPorts, err := sidecar.exposePorts(exposedPorts, name)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		maps.Copy(exposedPorts, newExposedPorts)
	}
	for _, rule := range b.HTTP.RoutingRules() {
		maps.Copy(exposedPorts, rule.exposePorts(exposedPorts, workloadName))
	}
	maps.Copy(exposedPorts, b.ImageConfig.exposePorts(exposedPorts, workloadName))
	portsForContainer, containerForPort := prepareParsedExposedPortsMap(exposedPorts)
	return ExposedPortsIndex{
		WorkloadName:      workloadName,
		PortsForContainer: portsForContainer,
		ContainerForPort:  containerForPort,
	}, nil
}
