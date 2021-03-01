// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

var (
	errUnmarshalBuildOpts 	= errors.New("cannot unmarshal build field into string or compose-style map")
	errUnmarshalCountOpts 	= errors.New(`cannot unmarshal "count" field to an integer or autoscaling configuration`)
	errUnmarshalEntryPoint 	= errors.New("cannot unmarshal entrypoint into string or slice of strings")
	errUnmarshalCommand 	= errors.New("cannot unmarshal command into string or slice of strings")
)

const defaultFluentbitImage = "amazon/aws-for-fluent-bit:latest"

var dockerfileDefaultName = "Dockerfile"

// WorkloadTypes holds all workload manifest types.
var WorkloadTypes = append(ServiceTypes, JobTypes...)

// Workload holds the basic data that every workload manifest file needs to have.
type Workload struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// Image represents the workload's container image.
type Image struct {
	Build    BuildArgsOrString `yaml:"build"`    // Build an image from a Dockerfile.
	Location *string           `yaml:"location"` // Use an existing image instead.
}

// GetLocation returns the location of the image.
func (i Image) GetLocation() string {
	return aws.StringValue(i.Location)
}

// BuildConfig populates a docker.BuildArguments struct from the fields available in the manifest.
// Prefer the following hierarchy:
// 1. Specific dockerfile, specific context
// 2. Specific dockerfile, context = dockerfile dir
// 3. "Dockerfile" located in context dir
// 4. "Dockerfile" located in ws root.
func (i *Image) BuildConfig(rootDirectory string) *DockerBuildArgs {
	df := i.dockerfile()
	ctx := i.context()
	dockerfile := aws.String(filepath.Join(rootDirectory, dockerfileDefaultName))
	context := aws.String(rootDirectory)

	if df != "" && ctx != "" {
		dockerfile = aws.String(filepath.Join(rootDirectory, df))
		context = aws.String(filepath.Join(rootDirectory, ctx))
	}
	if df != "" && ctx == "" {
		dockerfile = aws.String(filepath.Join(rootDirectory, df))
		context = aws.String(filepath.Join(rootDirectory, filepath.Dir(df)))
	}
	if df == "" && ctx != "" {
		dockerfile = aws.String(filepath.Join(rootDirectory, ctx, dockerfileDefaultName))
		context = aws.String(filepath.Join(rootDirectory, ctx))
	}
	return &DockerBuildArgs{
		Dockerfile: dockerfile,
		Context:    context,
		Args:       i.args(),
		Target:     i.target(),
		CacheFrom:  i.cacheFrom(),
	}
}

// dockerfile returns the path to the workload's Dockerfile. If no dockerfile is specified,
// returns "".
func (i *Image) dockerfile() string {
	// Prefer to use the "Dockerfile" string in BuildArgs. Otherwise,
	// "BuildString". If no dockerfile specified, return "".
	if i.Build.BuildArgs.Dockerfile != nil {
		return aws.StringValue(i.Build.BuildArgs.Dockerfile)
	}

	var dfPath string
	if i.Build.BuildString != nil {
		dfPath = aws.StringValue(i.Build.BuildString)
	}

	return dfPath
}

// context returns the build context directory if it exists, otherwise an empty string.
func (i *Image) context() string {
	return aws.StringValue(i.Build.BuildArgs.Context)
}

// args returns the args section, if it exists, to override args in the dockerfile.
// Otherwise it returns an empty map.
func (i *Image) args() map[string]string {
	return i.Build.BuildArgs.Args
}

// target returns the build target stage if it exists, otherwise nil.
func (i *Image) target() *string {
	return i.Build.BuildArgs.Target
}

// cacheFrom returns the cache from build section, if it exists.
// Otherwise it returns nil.
func (i *Image) cacheFrom() []string {
	return i.Build.BuildArgs.CacheFrom
}

// ImageOverride holds fields that override Dockerfile image defaults.
type ImageOverride struct {
	EntryPoint EntryPointOverride `yaml:"entrypoint"`
	Command    CommandOverride    `yaml:"command"`
}

// EntryPointOverride is a custom type which supports unmarshaling "entrypoint" yaml which
// can either be of type string or type slice of string.
type EntryPointOverride stringSliceOrString

// CommandOverride is a custom type which supports unmarshaling "command" yaml which
// can either be of type string or type slice of string.
type CommandOverride stringSliceOrString

// UnmarshalYAML overrides the default YAML unmarshaling logic for the EntryPointOverride
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (e *EntryPointOverride) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshalYAMLToStringSliceOrString((*stringSliceOrString)(e), unmarshal); err != nil {
		return errUnmarshalEntryPoint
	}
	return nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the CommandOverride
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (c *CommandOverride) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshalYAMLToStringSliceOrString((*stringSliceOrString)(c), unmarshal); err != nil {
		return errUnmarshalCommand
	}
	return nil
}

type stringSliceOrString struct {
	String      *string
	StringSlice []string
}

func unmarshalYAMLToStringSliceOrString(s *stringSliceOrString, unmarshal func(interface{}) error) error {
	if err := unmarshal(&s.StringSlice); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if s.StringSlice != nil {
		// Unmarshaled successfully to s.StringSlice, unset s.String, and return.
		s.String = nil
		return nil
	}

	return unmarshal(&s.String)
}

// BuildArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type DockerBuildArgs.
type BuildArgsOrString struct {
	BuildString *string
	BuildArgs   DockerBuildArgs
}

func (b *BuildArgsOrString) isEmpty() bool {
	if aws.StringValue(b.BuildString) == "" && b.BuildArgs.isEmpty() {
		return true
	}
	return false
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
		// Unmarshaled successfully to b.BuildArgs, unset b.BuildString, and return.
		b.BuildString = nil
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
	Target     *string           `yaml:"target,omitempty"`
	CacheFrom  []string          `yaml:"cache_from,omitempty"`
}

func (b *DockerBuildArgs) isEmpty() bool {
	if b.Context == nil && b.Dockerfile == nil && b.Args == nil && b.Target == nil && b.CacheFrom == nil {
		return true
	}
	return false
}

// Logging holds configuration for Firelens to route your logs.
type Logging struct {
	Image          *string           `yaml:"image"`
	Destination    map[string]string `yaml:"destination,flow"`
	EnableMetadata *bool             `yaml:"enableMetadata"`
	SecretOptions  map[string]string `yaml:"secretOptions"`
	ConfigFile     *string           `yaml:"configFilePath"`
}

// LogImage returns the default Fluent Bit image if not otherwise configured.
func (lc *Logging) LogImage() *string {
	if lc.Image == nil {
		return aws.String(defaultFluentbitImage)
	}
	return lc.Image
}

// GetEnableMetadata returns the configuration values and sane default for the EnableMEtadata field
func (lc *Logging) GetEnableMetadata() *string {
	if lc.EnableMetadata == nil {
		// Enable ecs log metadata by default.
		return aws.String("true")
	}
	return aws.String(strconv.FormatBool(*lc.EnableMetadata))
}

// SidecarConfig represents the configurable options for setting up a sidecar container.
type SidecarConfig struct {
	Port        *string             `yaml:"port"`
	Image       *string             `yaml:"image"`
	CredsParam  *string             `yaml:"credentialsParameter"`
	Variables   map[string]string   `yaml:"variables"`
	Secrets     map[string]string   `yaml:"secrets"`
	MountPoints []SidecarMountPoint `yaml:"mount_points"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU       *int              `yaml:"cpu"`
	Memory    *int              `yaml:"memory"`
	Count     Count             `yaml:"count"`
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
	Storage   Storage           `yaml:"storage"`
}

// WorkloadProps contains properties for creating a new workload manifest.
type WorkloadProps struct {
	Name       string
	Dockerfile string
	Image      string
}

// UnmarshalWorkload deserializes the YAML input stream into a workload manifest object.
// If an error occurs during deserialization, then returns the error.
// If the workload type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalWorkload(in []byte) (interface{}, error) {
	am := Workload{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, fmt.Errorf("unmarshal to workload manifest: %w", err)
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
		if m.BackendServiceConfig.ImageConfig.HealthCheck != nil {
			// Make sure that unset fields in the healthcheck gets a default value.
			m.BackendServiceConfig.ImageConfig.HealthCheck.applyIfNotSet(newDefaultContainerHealthCheck())
		}
		return m, nil
	case ScheduledJobType:
		m := newDefaultScheduledJob()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to scheduled job: %w", err)
		}
		return m, nil
	default:
		return nil, &ErrInvalidWorkloadType{Type: typeVal}
	}
}

func requiresBuild(image Image) (bool, error) {
	noBuild, noURL := image.Build.isEmpty(), image.Location == nil
	// Error if both of them are specified or neither is specified.
	if noBuild == noURL {
		return false, fmt.Errorf(`either "image.build" or "image.location" needs to be specified in the manifest`)
	}
	if image.Location == nil {
		return true, nil
	}
	return false, nil
}

func dockerfileBuildRequired(workloadType string, svc interface{}) (bool, error) {
	type manifest interface {
		BuildRequired() (bool, error)
	}
	mf, ok := svc.(manifest)
	if !ok {
		return false, fmt.Errorf("%s does not have required methods BuildRequired()", workloadType)
	}
	required, err := mf.BuildRequired()
	if err != nil {
		return false, fmt.Errorf("check if %s requires building from local Dockerfile: %w", workloadType, err)
	}
	return required, nil
}

func stringP(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func uint16P(n uint16) *uint16 {
	if n == 0 {
		return nil
	}
	return &n
}
