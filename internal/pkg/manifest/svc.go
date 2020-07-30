// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

const (
	// LoadBalancedWebServiceType is a web service with a load balancer and Fargate as compute.
	LoadBalancedWebServiceType = "Load Balanced Web Service"
	// BackendServiceType is a service that cannot be accessed from the internet but can be reached from other services.
	BackendServiceType = "Backend Service"

	defaultSidecarPort    = "80"
	defaultFluentbitImage = "amazon/aws-for-fluent-bit:latest"
)

var (
	errUnmarshalBuildOpts = errors.New("can't unmarshal build field into string or compose-style map")
	errNoDockerfile       = errors.New("must specify a Dockerfile path")
)

var dockerfileDefaultName = "Dockerfile"

// ServiceTypes are the supported service manifest types.
var ServiceTypes = []string{
	LoadBalancedWebServiceType,
	BackendServiceType,
}

// Service holds the basic data that every service manifest file needs to have.
type Service struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// ServiceImage represents the service's container image.
type ServiceImage struct {
	Build BuildArgsOrString `yaml:"build"` // Path to the Dockerfile.
}

// BuildConfig populates a docker.BuildArguments struct from the fields available in the manifest.
// Prefer the following hierarchy:
// 1. Specific dockerfile, specific context
// 2. Specific dockerfile, context = dockerfile dir
// 3. "Dockerfile" located in context dir
// 4. "Dockerfile" located in ws root.
func (s *ServiceImage) BuildConfig(rootDirectory string) *docker.BuildArguments {
	df := s.dockerfile()
	ctx := s.context()
	if df != "" && ctx != "" {
		return &docker.BuildArguments{
			Dockerfile: filepath.Join(rootDirectory, df),
			Context:    filepath.Join(rootDirectory, ctx),
			Args:       s.args(),
		}
	}
	if df != "" && ctx == "" {
		return &docker.BuildArguments{
			Dockerfile: filepath.Join(rootDirectory, df),
			Context:    filepath.Dir(df),
			Args:       s.args(),
		}
	}
	if df == "" && ctx != "" {
		return &docker.BuildArguments{
			Dockerfile: filepath.Join(ctx, dockerfileDefaultName),
			Context:    filepath.Join(rootDirectory, ctx),
			Args:       s.args(),
		}
	}
	return &docker.BuildArguments{
		Dockerfile: filepath.Join(rootDirectory, dockerfileDefaultName),
		Context:    rootDirectory,
		Args:       s.args(),
	}
}

// dockerfile returns the path to the service's Dockerfile. If no dockerfile is specified,
// returns "".
func (s *ServiceImage) dockerfile() string {
	// Prefer to use the "Dockerfile" string in BuildArgs. Otherwise,
	// "BuildString". If no dockerfile specified, return "".
	if s.Build.BuildArgs.Dockerfile != nil {
		return aws.StringValue(s.Build.BuildArgs.Dockerfile)
	}

	var dfPath string
	if s.Build.BuildString != nil {
		dfPath = aws.StringValue(s.Build.BuildString)
	}

	return dfPath
}

// context returns the build context directory if it exists, otherwise an empty string.
func (s *ServiceImage) context() string {
	return aws.StringValue(s.Build.BuildArgs.Context)
}

// args returns the args section, if it exists, to override args in the dockerfile.
// Otherwise it returns an empty map.
func (s *ServiceImage) args() map[string]string {
	return s.Build.BuildArgs.Args
}

// BuildArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type DockerBuildArgs.
type BuildArgsOrString struct {
	BuildString *string
	BuildArgs   DockerBuildArgs
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the BuildArgsOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (b *BuildArgsOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&b.BuildArgs); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !b.BuildArgs.isEmpty() {
		// Unmarshaled successfully to b.BuildArgs, return.
		return nil
	}

	if err := unmarshal(&b.BuildString); err != nil {
		return errUnmarshalBuildOpts
	}
	return nil
}

// DockerBuildArgs represents the options specifiable under the "build" field
// of Docker Compose services. For more information, see:
// https://docs.docker.com/compose/compose-file/#build
type DockerBuildArgs struct {
	Context    *string           `yaml:"context,omitempty"`
	Dockerfile *string           `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
}

func (b *DockerBuildArgs) isEmpty() bool {
	if b.Context == nil && b.Dockerfile == nil && b.Args == nil {
		return true
	}
	return false
}

// ServiceImageWithPort represents a container image with an exposed port.
type ServiceImageWithPort struct {
	ServiceImage `yaml:",inline"`
	Port         *uint16 `yaml:"port"`
}

// LogConfig holds configuration for Firelens to route your logs.
type LogConfig struct {
	Image          *string           `yaml:"image"`
	Destination    map[string]string `yaml:"destination,flow"`
	EnableMetadata *bool             `yaml:"enableMetadata"`
	SecretOptions  map[string]string `yaml:"secretOptions"`
	ConfigFile     *string           `yaml:"configFilePath"`
}

func (lc *LogConfig) logConfigOpts() *template.LogConfigOpts {
	return &template.LogConfigOpts{
		Image:          lc.image(),
		ConfigFile:     lc.ConfigFile,
		EnableMetadata: lc.enableMetadata(),
		Destination:    lc.Destination,
		SecretOptions:  lc.SecretOptions,
	}
}

func (lc *LogConfig) image() *string {
	if lc.Image == nil {
		return aws.String(defaultFluentbitImage)
	}
	return lc.Image
}

func (lc *LogConfig) enableMetadata() *string {
	if lc.EnableMetadata == nil {
		// Enable ecs log metadata by default.
		return aws.String("true")
	}
	return aws.String(strconv.FormatBool(*lc.EnableMetadata))
}

// Sidecar holds configuration for all sidecar containers in a service.
type Sidecar struct {
	Sidecars map[string]*SidecarConfig `yaml:"sidecars"`
}

// SidecarsOpts converts the service's sidecar configuration into a format parsable by the templates pkg.
func (s *Sidecar) SidecarsOpts() ([]*template.SidecarOpts, error) {
	if s.Sidecars == nil {
		return nil, nil
	}
	var sidecars []*template.SidecarOpts
	for name, config := range s.Sidecars {
		port, protocol, err := parsePortMapping(config.Port)
		if err != nil {
			return nil, err
		}
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:       aws.String(name),
			Image:      config.Image,
			Port:       port,
			Protocol:   protocol,
			CredsParam: config.CredsParam,
		})
	}
	return sidecars, nil
}

// SidecarConfig represents the configurable options for setting up a sidecar container.
type SidecarConfig struct {
	Port       *string `yaml:"port"`
	Image      *string `yaml:"image"`
	CredsParam *string `yaml:"credentialsParameter"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU       *int              `yaml:"cpu"`
	Memory    *int              `yaml:"memory"`
	Count     *int              `yaml:"count"` // 0 is a valid value, so we want the default value to be nil.
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
}

// ServiceProps contains properties for creating a new service manifest.
type ServiceProps struct {
	Name       string
	Dockerfile string
}

// UnmarshalService deserializes the YAML input stream into a service manifest object.
// If an error occurs during deserialization, then returns the error.
// If the service type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalService(in []byte) (interface{}, error) {
	am := Service{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, fmt.Errorf("unmarshal to service manifest: %w", err)
	}
	typeVal := aws.StringValue(am.Type)

	switch typeVal {
	case LoadBalancedWebServiceType:
		m := newDefaultLoadBalancedWebService()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to load balanced web service: %w", err)
		}
		return m, nil
	case BackendServiceType:
		m := newDefaultBackendService()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to backend service: %w", err)
		}
		if m.BackendServiceConfig.Image.HealthCheck != nil {
			// Make sure that unset fields in the healthcheck gets a default value.
			m.BackendServiceConfig.Image.HealthCheck.applyIfNotSet(newDefaultContainerHealthCheck())
		}
		return m, nil
	default:
		return nil, &ErrInvalidSvcManifestType{Type: typeVal}
	}
}

func durationp(v time.Duration) *time.Duration {
	return &v
}

func boolpcopy(v *bool) *bool {
	if v == nil {
		return nil
	}
	return aws.Bool(*v)
}

func stringpcopy(v *string) *string {
	if v == nil {
		return nil
	}
	return aws.String(*v)
}

func intpcopy(v *int) *int {
	if v == nil {
		return nil
	}
	return aws.Int(*v)
}

// Valid sidecar portMapping example: 2000/udp, or 2000 (default to be tcp).
func parsePortMapping(s *string) (port *string, protocol *string, err error) {
	if s == nil {
		// default port for sidecar container to be 80.
		return aws.String(defaultSidecarPort), nil, nil
	}
	portProtocol := strings.Split(*s, "/")
	switch len(portProtocol) {
	case 1:
		return aws.String(portProtocol[0]), nil, nil
	case 2:
		return aws.String(portProtocol[0]), aws.String(portProtocol[1]), nil
	default:
		return nil, nil, fmt.Errorf("cannot parse port mapping from %s", *s)
	}
}
