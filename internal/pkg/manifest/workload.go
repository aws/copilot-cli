// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/dustin/go-humanize/english"

	"github.com/google/shlex"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

const (
	defaultFluentbitImage = "amazon/aws-for-fluent-bit:latest"
	defaultDockerfileName = "Dockerfile"

	// AWS VPC subnet placement options.
	PublicSubnetPlacement  = "public"
	PrivateSubnetPlacement = "private"
)

var (
	// WorkloadTypes holds all workload manifest types.
	WorkloadTypes = append(ServiceTypes, JobTypes...)

	// All placement options.
	subnetPlacements = []string{PublicSubnetPlacement, PrivateSubnetPlacement}

	validPlatforms        = []string{dockerengine.DockerBuildPlatform(dockerengine.LinuxOS, dockerengine.Amd64Arch)}
	validOperatingSystems = []string{dockerengine.LinuxOS}
	validArchitectures    = []string{dockerengine.Amd64Arch}

	// Error definitions.
	errUnmarshalBuildOpts    = errors.New("unable to unmarshal build field into string or compose-style map")
	errUnmarshalPlatformOpts = errors.New("unable to unmarshal platform field into string or compose-style map")
	errUnmarshalCountOpts    = errors.New(`unable to unmarshal "count" field to an integer or autoscaling configuration`)
	errUnmarshalRangeOpts    = errors.New(`unable to unmarshal "range" field`)
	errUnmarshalExec         = errors.New("unable to unmarshal exec field into boolean or exec configuration")
	errUnmarshalEntryPoint   = errors.New("unable to unmarshal entrypoint into string or slice of strings")
	errUnmarshalCommand      = errors.New("unable to unmarshal command into string or slice of strings")
)

// WorkloadManifest represents a workload manifest.
type WorkloadManifest interface {
	ApplyEnv(envName string) (WorkloadManifest, error)
}

// WorkloadProps contains properties for creating a new workload manifest.
type WorkloadProps struct {
	Name       string
	Dockerfile string
	Image      string
}

// Workload holds the basic data that every workload manifest file needs to have.
type Workload struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// OverrideRule holds the manifest overriding rule for CloudFormation template.
type OverrideRule struct {
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

// Image represents the workload's container image.
type Image struct {
	Build        BuildArgsOrString `yaml:"build"`           // Build an image from a Dockerfile.
	Location     *string           `yaml:"location"`        // Use an existing image instead.
	Credentials  *string           `yaml:"credentials"`     // ARN of the secret containing the private repository credentials.
	DockerLabels map[string]string `yaml:"labels,flow"`     // Apply Docker labels to the container at runtime.
	DependsOn    map[string]string `yaml:"depends_on,flow"` // Add any sidecar dependencies.
}

// ImageWithHealthcheck represents a container image with health check.
type ImageWithHealthcheck struct {
	Image       `yaml:",inline"`
	HealthCheck ContainerHealthCheck `yaml:"healthcheck"`
}

// ImageWithPortAndHealthcheck represents a container image with an exposed port and health check.
type ImageWithPortAndHealthcheck struct {
	ImageWithPort `yaml:",inline"`
	HealthCheck   ContainerHealthCheck `yaml:"healthcheck"`
}

// ImageWithPort represents a container image with an exposed port.
type ImageWithPort struct {
	Image `yaml:",inline"`
	Port  *uint16 `yaml:"port"`
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
	dockerfile := aws.String(filepath.Join(rootDirectory, defaultDockerfileName))
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
		dockerfile = aws.String(filepath.Join(rootDirectory, ctx, defaultDockerfileName))
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

// ToStringSlice converts an EntryPointOverride to a slice of string using shell-style rules.
func (e *EntryPointOverride) ToStringSlice() ([]string, error) {
	out, err := toStringSlice((*stringSliceOrString)(e))
	if err != nil {
		return nil, err
	}
	return out, nil
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

// ToStringSlice converts an CommandOverride to a slice of string using shell-style rules.
func (c *CommandOverride) ToStringSlice() ([]string, error) {
	out, err := toStringSlice((*stringSliceOrString)(c))
	if err != nil {
		return nil, err
	}
	return out, nil
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

func toStringSlice(s *stringSliceOrString) ([]string, error) {
	if s.StringSlice != nil {
		return s.StringSlice, nil
	}

	if s.String == nil {
		return nil, nil
	}

	out, err := shlex.Split(*s.String)
	if err != nil {
		return nil, fmt.Errorf("convert string into tokens using shell-style rules: %w", err)
	}

	return out, nil
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

// ExecuteCommand is a custom type which supports unmarshaling yaml which
// can either be of type bool or type ExecuteCommandConfig.
type ExecuteCommand struct {
	Enable *bool
	Config ExecuteCommandConfig
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ExecuteCommand
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (e *ExecuteCommand) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&e.Config); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !e.Config.IsEmpty() {
		return nil
	}

	if err := unmarshal(&e.Enable); err != nil {
		return errUnmarshalExec
	}
	return nil
}

// ExecuteCommandConfig represents the configuration for ECS Execute Command.
type ExecuteCommandConfig struct {
	Enable *bool `yaml:"enable"`
	// Reserved for future use.
}

// IsEmpty returns whether ExecuteCommandConfig is empty.
func (e ExecuteCommandConfig) IsEmpty() bool {
	return e.Enable == nil
}

// Logging holds configuration for Firelens to route your logs.
type Logging struct {
	Retention      *int              `yaml:"retention"`
	Image          *string           `yaml:"image"`
	Destination    map[string]string `yaml:"destination,flow"`
	EnableMetadata *bool             `yaml:"enableMetadata"`
	SecretOptions  map[string]string `yaml:"secretOptions"`
	ConfigFile     *string           `yaml:"configFilePath"`
}

// IsEmpty returns empty if the struct has all zero members.
func (lc *Logging) IsEmpty() bool {
	return lc.Image == nil && lc.Destination == nil && lc.EnableMetadata == nil && lc.SecretOptions == nil && lc.ConfigFile == nil
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
	Port          *string              `yaml:"port"`
	Image         *string              `yaml:"image"`
	Essential     *bool                `yaml:"essential"`
	CredsParam    *string              `yaml:"credentialsParameter"`
	Variables     map[string]string    `yaml:"variables"`
	Secrets       map[string]string    `yaml:"secrets"`
	MountPoints   []SidecarMountPoint  `yaml:"mount_points"`
	DockerLabels  map[string]string    `yaml:"labels"`
	DependsOn     map[string]string    `yaml:"depends_on"`
	HealthCheck   ContainerHealthCheck `yaml:"healthcheck"`
	ImageOverride `yaml:",inline"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU            *int                 `yaml:"cpu"`
	Memory         *int                 `yaml:"memory"`
	Platform       PlatformArgsOrString `yaml:"platform,omitempty"`
	Count          Count                `yaml:"count"`
	ExecuteCommand ExecuteCommand       `yaml:"exec"`
	Variables      map[string]string    `yaml:"variables"`
	Secrets        map[string]string    `yaml:"secrets"`
	Storage        Storage              `yaml:"storage"`
}

// TaskPlatform returns the platform for the service.
func (t *TaskConfig) TaskPlatform() (*string, error) {
	if t.Platform.PlatformString == nil {
		return nil, nil
	}
	return t.Platform.PlatformString, nil
}

// PublishConfig represents the configurable options for setting up publishers.
type PublishConfig struct {
	Topics []Topic `yaml:"topics"`
}

// Topic represents the configurable options for setting up a SNS Topic.
type Topic struct {
	Name *string `yaml:"name"`
}

// NetworkConfig represents options for network connection to AWS resources within a VPC.
type NetworkConfig struct {
	VPC vpcConfig `yaml:"vpc"`
}

// IsEmpty returns empty if the struct has all zero members.
func (c *NetworkConfig) IsEmpty() bool {
	return c.VPC.isEmpty()
}

// UnmarshalYAML ensures that a NetworkConfig always defaults to public subnets.
// If the user specified a placement that's not valid then throw an error.
func (c *NetworkConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type networkWithDefaults NetworkConfig
	defaultVPCConf := vpcConfig{
		Placement: stringP(PublicSubnetPlacement),
	}
	conf := networkWithDefaults{
		VPC: defaultVPCConf,
	}
	if err := unmarshal(&conf); err != nil {
		return err
	}
	if conf.VPC.isEmpty() { // If after unmarshaling the user did not specify VPC configuration then reset it to public.
		conf.VPC = defaultVPCConf
	}
	if !conf.VPC.isValidPlacement() {
		return fmt.Errorf("field '%s' is '%v' must be one of %#v", "network.vpc.placement", aws.StringValue(conf.VPC.Placement), subnetPlacements)
	}
	*c = NetworkConfig(conf)
	return nil
}

// vpcConfig represents the security groups and subnets attached to a task.
type vpcConfig struct {
	Placement      *string  `yaml:"placement"`
	SecurityGroups []string `yaml:"security_groups"`
}

func (c *vpcConfig) isEmpty() bool {
	return c.Placement == nil && c.SecurityGroups == nil
}

func (c *vpcConfig) isValidPlacement() bool {
	if c.Placement == nil {
		return false
	}
	for _, allowed := range subnetPlacements {
		if *c.Placement == allowed {
			return true
		}
	}
	return false
}

// UnmarshalWorkload deserializes the YAML input stream into a workload manifest object.
// If an error occurs during deserialization, then returns the error.
// If the workload type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalWorkload(in []byte) (WorkloadManifest, error) {
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
	case RequestDrivenWebServiceType:
		m := newDefaultRequestDrivenWebService()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to request-driven web service: %w", err)
		}
		return m, nil
	case BackendServiceType:
		m := newDefaultBackendService()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to backend service: %w", err)
		}
		return m, nil
	case WorkerServiceType:
		m := newDefaultWorkerService()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to worker service: %w", err)
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

// ContainerHealthCheck holds the configuration to determine if the service container is healthy.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-healthcheck.html
type ContainerHealthCheck struct {
	Command     []string       `yaml:"command"`
	Interval    *time.Duration `yaml:"interval"`
	Retries     *int           `yaml:"retries"`
	Timeout     *time.Duration `yaml:"timeout"`
	StartPeriod *time.Duration `yaml:"start_period"`
}

// NewDefaultContainerHealthCheck returns container health check configuration
// that's identical to a load balanced web service's defaults.
func NewDefaultContainerHealthCheck() *ContainerHealthCheck {
	return &ContainerHealthCheck{
		Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
		Interval:    durationp(10 * time.Second),
		Retries:     aws.Int(2),
		Timeout:     durationp(5 * time.Second),
		StartPeriod: durationp(0 * time.Second),
	}
}

// IsEmpty checks if the health check is empty.
func (hc ContainerHealthCheck) IsEmpty() bool {
	return hc.Command == nil && hc.Interval == nil && hc.Retries == nil && hc.Timeout == nil && hc.StartPeriod == nil
}

// ApplyIfNotSet changes the healthcheck's fields only if they were not set and the other healthcheck has them set.
func (hc *ContainerHealthCheck) ApplyIfNotSet(other *ContainerHealthCheck) {
	if hc.Command == nil && other.Command != nil {
		hc.Command = other.Command
	}
	if hc.Interval == nil && other.Interval != nil {
		hc.Interval = other.Interval
	}
	if hc.Retries == nil && other.Retries != nil {
		hc.Retries = other.Retries
	}
	if hc.Timeout == nil && other.Timeout != nil {
		hc.Timeout = other.Timeout
	}
	if hc.StartPeriod == nil && other.StartPeriod != nil {
		hc.StartPeriod = other.StartPeriod
	}
}

// PlatformArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type PlatformArgs.
type PlatformArgsOrString struct {
	PlatformString *string
	PlatformArgs   PlatformArgs
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the PlatformArgsOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (p *PlatformArgsOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&p.PlatformArgs); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !p.PlatformArgs.isEmpty() {
		if !p.PlatformArgs.bothSpecified() {
			return errors.New(`fields 'osfamily' and 'architecture' must either both be specified or both be empty.`)
		}
		if err := validateOS(p.PlatformArgs.OSFamily); err != nil {
			return fmt.Errorf("validate OS: %w", err)
		}
		if err := validateArch(p.PlatformArgs.Arch); err != nil {
			return fmt.Errorf("validate arch: %w", err)
		}
		// Unmarshaled successfully to p.PlatformArgs, unset p.PlatformString, and return.
		p.PlatformString = nil
		return nil
	}
	if err := unmarshal(&p.PlatformString); err != nil {
		return errUnmarshalPlatformOpts
	}
	if err := validatePlatform(p.PlatformString); err != nil {
		return fmt.Errorf("validate platform: %w", err)
	}
	return nil
}

// PlatformArgs represents the specifics of a target OS.
type PlatformArgs struct {
	OSFamily *string `yaml:"osfamily,omitempty"`
	Arch     *string `yaml:"architecture,omitempty"`
}

func (p *PlatformArgs) isEmpty() bool {
	return p.OSFamily == nil && p.Arch == nil
}

func (p *PlatformArgs) bothSpecified() bool {
	return (p.OSFamily != nil) && (p.Arch != nil)
}

func validatePlatform(platform *string) error {
	if platform == nil {
		return nil
	}
	for _, validPlatform := range validPlatforms {
		if aws.StringValue(platform) == validPlatform {
			return nil
		}
	}
	return fmt.Errorf("platform %s is invalid; %s: %s", aws.StringValue(platform), english.PluralWord(len(validPlatforms), "the valid platform is", "valid platforms are"), english.WordSeries(validPlatforms, "and"))
}

func validateOS(os *string) error {
	if os == nil {
		return nil
	}
	for _, validOS := range validOperatingSystems {
		if aws.StringValue(os) == validOS {
			return nil
		}
	}
	return fmt.Errorf("OS %s is invalid; %s: %s", aws.StringValue(os), english.PluralWord(len(validOperatingSystems), "the valid operating system is", "valid operating systems are"), english.WordSeries(validOperatingSystems, "and"))
}

func validateArch(arch *string) error {
	if arch == nil {
		return nil
	}
	for _, validArch := range validArchitectures {
		if aws.StringValue(arch) == validArch {
			return nil
		}
	}
	return fmt.Errorf("architecture %s is invalid; %s: %s", aws.StringValue(arch), english.PluralWord(len(validArchitectures), "the valid architecture is", "valid architectures are"), english.WordSeries(validArchitectures, "and"))
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
