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
	errUnmarshalCountOpts = errors.New(`unmarshal "count" field to an integer or autoscaling configuration`)
)

var dockerfileDefaultName = "Dockerfile"

// ServiceTypes are the supported service manifest types.
var ServiceTypes = []string{
	LoadBalancedWebServiceType,
	BackendServiceType,
}

const (
	// ScheduledJobType is a job which is run periodically in a given environment.
	ScheduledJobType = "Scheduled Job"
)

// JobTypes are the supported job manifest types. 
var JobTypes = []string{
	ScheduledJobType,
}

// Range is a number range with maximum and minimum values.
type Range string

// Parse parses Range string and returns the min and max values.
// For example: 1-100 returns 1 and 100.
func (r Range) Parse() (min int, max int, err error) {
	minMax := strings.Split(string(r), "-")
	if len(minMax) != 2 {
		return 0, 0, fmt.Errorf("invalid range value %s. Should be in format of ${min}-${max}", string(r))
	}
	min, err = strconv.Atoi(minMax[0])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert minimum value %s to integer", minMax[0])
	}
	max, err = strconv.Atoi(minMax[1])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert maximum value %s to integer", minMax[1])
	}
	return min, max, nil
}

// Service holds the basic data that every service manifest file needs to have.
type Service struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// IsService returns true if a manifest's type is one of the supported service types.
func (s *Service) IsService() bool {
	for _, mftType := range ServiceTypes {
		if aws.StringValue(s.Type) == mftType {
			return true
		}
	}
	return false
}

// IsJob returns true if a manifest's type is one of the supported job types.
func (s *Service) IsJob() bool {
	for _, mftType := range JobTypes {
		if aws.StringValue(s.Type) == mftType {
			return true
		}
	}
	return false
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
func (s *ServiceImage) BuildConfig(rootDirectory string) *DockerBuildArgs {
	df := s.dockerfile()
	ctx := s.context()
	if df != "" && ctx != "" {
		return &DockerBuildArgs{
			Dockerfile: aws.String(filepath.Join(rootDirectory, df)),
			Context:    aws.String(filepath.Join(rootDirectory, ctx)),
			Args:       s.args(),
		}
	}
	if df != "" && ctx == "" {
		return &DockerBuildArgs{
			Dockerfile: aws.String(filepath.Join(rootDirectory, df)),
			Context:    aws.String(filepath.Join(rootDirectory, filepath.Dir(df))),
			Args:       s.args(),
		}
	}
	if df == "" && ctx != "" {
		return &DockerBuildArgs{
			Dockerfile: aws.String(filepath.Join(rootDirectory, ctx, dockerfileDefaultName)),
			Context:    aws.String(filepath.Join(rootDirectory, ctx)),
			Args:       s.args(),
		}
	}
	return &DockerBuildArgs{
		Dockerfile: aws.String(filepath.Join(rootDirectory, dockerfileDefaultName)),
		Context:    aws.String(rootDirectory),
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

// Options converts the service's sidecar configuration into a format parsable by the templates pkg.
func (s *Sidecar) Options() ([]*template.SidecarOpts, error) {
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
	Count     Count             `yaml:"count"`
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
}

// Count is a custom type which supports unmarshaling yaml which
// can either be of type int or type Autoscaling.
type Count struct {
	Value       *int        // 0 is a valid value, so we want the default value to be nil.
	Autoscaling Autoscaling // Mutually exclusive with Value.
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Count
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (a *Count) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&a.Autoscaling); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !a.Autoscaling.IsEmpty() {
		return nil
	}

	if err := unmarshal(&a.Value); err != nil {
		return errUnmarshalCountOpts
	}
	return nil
}

// Autoscaling represents the configurable options for Auto Scaling.
type Autoscaling struct {
	Range        Range          `yaml:"range"`
	CPU          *int           `yaml:"cpu_percentage"`
	Memory       *int           `yaml:"memory_percentage"`
	Requests     *int           `yaml:"requests"`
	ResponseTime *time.Duration `yaml:"response_time"`
}

// Options converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func (a *Autoscaling) Options() (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	min, max, err := a.Range.Parse()
	if err != nil {
		return nil, err
	}
	autoscalingOpts := template.AutoscalingOpts{
		MinCapacity: &min,
		MaxCapacity: &max,
	}
	if a.CPU != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU))
	}
	if a.Memory != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory))
	}
	if a.Requests != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests))
	}
	if a.ResponseTime != nil {
		responseTime := float64(*a.ResponseTime) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	return &autoscalingOpts, nil
}

// IsEmpty returns whether Autoscaling is empty.
func (a *Autoscaling) IsEmpty() bool {
	return a.Range == "" && a.CPU == nil && a.Memory == nil &&
		a.Requests == nil && a.ResponseTime == nil
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
