// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/google/shlex"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

const (
	defaultDockerfileName = "Dockerfile"
)

// SQS Queue field options.
const (
	sqsFIFOThroughputLimitPerMessageGroupID = "perMessageGroupId"
	sqsFIFOThroughputLimitPerQueue          = "perQueue"
	sqsDeduplicationScopeMessageGroup       = "messageGroup"
	sqsDeduplicationScopeQueue              = "queue"
)

// AWS VPC subnet placement options.
const (
	PublicSubnetPlacement  = PlacementString("public")
	PrivateSubnetPlacement = PlacementString("private")
)

// All placement options.
var (
	subnetPlacements = []string{string(PublicSubnetPlacement), string(PrivateSubnetPlacement)}
)

// Error definitions.
var (
	ErrAppRunnerInvalidPlatformWindows = errors.New("Windows is not supported for App Runner services")

	errUnmarshalBuildOpts          = errors.New("unable to unmarshal build field into string or compose-style map")
	errUnmarshalPlatformOpts       = errors.New("unable to unmarshal platform field into string or compose-style map")
	errUnmarshalSecurityGroupOpts  = errors.New(`unable to unmarshal "security_groups" field into slice of strings or compose-style map`)
	errUnmarshalPlacementOpts      = errors.New("unable to unmarshal placement field into string or compose-style map")
	errUnmarshalServiceConnectOpts = errors.New(`unable to unmarshal "connect" field into boolean or compose-style map`)
	errUnmarshalSubnetsOpts        = errors.New("unable to unmarshal subnets field into string slice or compose-style map")
	errUnmarshalCountOpts          = errors.New(`unable to unmarshal "count" field to an integer or autoscaling configuration`)
	errUnmarshalRangeOpts          = errors.New(`unable to unmarshal "range" field`)

	errUnmarshalExec       = errors.New(`unable to unmarshal "exec" field into boolean or exec configuration`)
	errUnmarshalEntryPoint = errors.New(`unable to unmarshal "entrypoint" into string or slice of strings`)
	errUnmarshalAlias      = errors.New(`unable to unmarshal "alias" into advanced alias map, string, or slice of strings`)
	errUnmarshalCommand    = errors.New(`unable to unmarshal "command" into string or slice of strings`)
)

// DynamicWorkload represents a dynamically populated workload.
type DynamicWorkload interface {
	ApplyEnv(envName string) (DynamicWorkload, error)
	Validate() error
	RequiredEnvironmentFeatures() []string
	Load(sess *session.Session) error
	Manifest() any
}

type workloadManifest interface {
	validate() error
	applyEnv(envName string) (workloadManifest, error)
	requiredEnvironmentFeatures() []string
	subnets() *SubnetListOrArgs
}

// UnmarshalWorkload deserializes the YAML input stream into a workload manifest object.
// If an error occurs during deserialization, then returns the error.
// If the workload type in the manifest is invalid, then returns an ErrInvalidmanifestinfo.
func UnmarshalWorkload(in []byte) (DynamicWorkload, error) {
	am := Workload{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, fmt.Errorf("unmarshal to workload manifest: %w", err)
	}
	typeVal := aws.StringValue(am.Type)
	var m workloadManifest
	switch typeVal {
	case manifestinfo.LoadBalancedWebServiceType:
		m = newDefaultLoadBalancedWebService()
	case manifestinfo.RequestDrivenWebServiceType:
		m = newDefaultRequestDrivenWebService()
	case manifestinfo.BackendServiceType:
		m = newDefaultBackendService()
	case manifestinfo.WorkerServiceType:
		m = newDefaultWorkerService()
	case manifestinfo.StaticSiteType:
		m = newDefaultStaticSite()
	case manifestinfo.ScheduledJobType:
		m = newDefaultScheduledJob()
	default:
		return nil, &ErrInvalidWorkloadType{Type: typeVal}
	}
	if err := yaml.Unmarshal(in, m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest for %s: %w", typeVal, err)
	}
	return newDynamicWorkloadManifest(m), nil
}

// WorkloadProps contains properties for creating a new workload manifest.
type WorkloadProps struct {
	Name                    string
	Dockerfile              string
	Image                   string
	PrivateOnlyEnvironments []string
}

// Workload holds the basic data that every workload manifest file needs to have.
type Workload struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// Image represents the workload's container image.
type Image struct {
	ImageLocationOrBuild `yaml:",inline"`
	Credentials          *string           `yaml:"credentials"`     // ARN of the secret containing the private repository credentials.
	DockerLabels         map[string]string `yaml:"labels,flow"`     // Apply Docker labels to the container at runtime.
	DependsOn            DependsOn         `yaml:"depends_on,flow"` // Add any sidecar dependencies.
}

// ImageLocationOrBuild represents the docker build arguments and location of the existing image.
type ImageLocationOrBuild struct {
	Build    BuildArgsOrString `yaml:"build"`    // Build an image from a Dockerfile.
	Location *string           `yaml:"location"` // Use an existing image instead.
}

// DependsOn represents container dependency for a container.
type DependsOn map[string]string

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Image
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (i *Image) UnmarshalYAML(value *yaml.Node) error {
	type image Image
	if err := value.Decode((*image)(i)); err != nil {
		return err
	}
	if !i.Build.isEmpty() && i.Location != nil {
		return &errFieldMutualExclusive{
			firstField:  "build",
			secondField: "location",
			mustExist:   true,
		}
	}
	return nil
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
func (i *ImageLocationOrBuild) BuildConfig(rootDirectory string) *DockerBuildArgs {
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
func (i *ImageLocationOrBuild) dockerfile() string {
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
func (i *ImageLocationOrBuild) context() string {
	return aws.StringValue(i.Build.BuildArgs.Context)
}

// args returns the args section, if it exists, to override args in the dockerfile.
// Otherwise it returns an empty map.
func (i *ImageLocationOrBuild) args() map[string]string {
	return i.Build.BuildArgs.Args
}

// target returns the build target stage if it exists, otherwise nil.
func (i *ImageLocationOrBuild) target() *string {
	return i.Build.BuildArgs.Target
}

// cacheFrom returns the cache from build section, if it exists.
// Otherwise it returns nil.
func (i *ImageLocationOrBuild) cacheFrom() []string {
	return i.Build.BuildArgs.CacheFrom
}

// ImageOverride holds fields that override Dockerfile image defaults.
type ImageOverride struct {
	EntryPoint EntryPointOverride `yaml:"entrypoint"`
	Command    CommandOverride    `yaml:"command"`
}

// StringSliceOrShellString is either a slice of string or a string using shell-style rules.
type stringSliceOrShellString StringSliceOrString

// EntryPointOverride is a custom type which supports unmarshalling "entrypoint" yaml which
// can either be of type string or type slice of string.
type EntryPointOverride stringSliceOrShellString

// CommandOverride is a custom type which supports unmarshalling "command" yaml which
// can either be of type string or type slice of string.
type CommandOverride stringSliceOrShellString

// UnmarshalYAML overrides the default YAML unmarshalling logic for the EntryPointOverride
// struct, allowing it to be unmarshalled into a string slice or a string.
// This method implements the yaml.Unmarshaler (v3) interface.
func (e *EntryPointOverride) UnmarshalYAML(value *yaml.Node) error {
	if err := (*StringSliceOrString)(e).UnmarshalYAML(value); err != nil {
		return errUnmarshalEntryPoint
	}
	return nil
}

// ToStringSlice converts an EntryPointOverride to a slice of string using shell-style rules.
func (e *EntryPointOverride) ToStringSlice() ([]string, error) {
	out, err := (*stringSliceOrShellString)(e).toStringSlice()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the CommandOverride
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (c *CommandOverride) UnmarshalYAML(value *yaml.Node) error {
	if err := (*StringSliceOrString)(c).UnmarshalYAML(value); err != nil {
		return errUnmarshalCommand
	}
	return nil
}

// ToStringSlice converts an CommandOverride to a slice of string using shell-style rules.
func (c *CommandOverride) ToStringSlice() ([]string, error) {
	out, err := (*stringSliceOrShellString)(c).toStringSlice()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StringSliceOrString is a custom type that can either be of type string or type slice of string.
type StringSliceOrString struct {
	String      *string
	StringSlice []string
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the StringSliceOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (s *StringSliceOrString) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.StringSlice); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}

	if s.StringSlice != nil {
		// Unmarshaled successfully to s.StringSlice, unset s.String, and return.
		s.String = nil
		return nil
	}

	return value.Decode(&s.String)
}

func (s *StringSliceOrString) isEmpty() bool {
	return s.String == nil && len(s.StringSlice) == 0
}

// ToStringSlice converts an StringSliceOrString to a slice of string.
func (s *StringSliceOrString) ToStringSlice() []string {
	if s.StringSlice != nil {
		return s.StringSlice
	}

	if s.String == nil {
		return nil
	}
	return []string{*s.String}
}

func (s *stringSliceOrShellString) toStringSlice() ([]string, error) {
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
// This method implements the yaml.Unmarshaler (v3) interface.
func (b *BuildArgsOrString) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&b.BuildArgs); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}

	if !b.BuildArgs.isEmpty() {
		// Unmarshaled successfully to b.BuildArgs, unset b.BuildString, and return.
		b.BuildString = nil
		return nil
	}

	if err := value.Decode(&b.BuildString); err != nil {
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

// PublishConfig represents the configurable options for setting up publishers.
type PublishConfig struct {
	Topics []Topic `yaml:"topics"`
}

// Topic represents the configurable options for setting up a SNS Topic.
type Topic struct {
	Name *string                      `yaml:"name"`
	FIFO FIFOTopicAdvanceConfigOrBool `yaml:"fifo"`
}

// FIFOTopicAdvanceConfigOrBool represents the configurable options for fifo topics.
type FIFOTopicAdvanceConfigOrBool struct {
	Enable   *bool
	Advanced FIFOTopicAdvanceConfig
}

// IsEmpty returns true if the FifoAdvanceConfigOrBool struct has all nil values.
func (f *FIFOTopicAdvanceConfigOrBool) IsEmpty() bool {
	return f.Enable == nil && f.Advanced.IsEmpty()
}

// IsEnabled returns true if the FIFO is enabled on the SQS queue.
func (f *FIFOTopicAdvanceConfigOrBool) IsEnabled() bool {
	return aws.BoolValue(f.Enable) || !f.Advanced.IsEmpty()
}

// FIFOTopicAdvanceConfig represents the advanced fifo topic config.
type FIFOTopicAdvanceConfig struct {
	ContentBasedDeduplication *bool `yaml:"content_based_deduplication"`
}

// IsEmpty returns true if the FifoAdvanceConfig struct has all nil values.
func (a *FIFOTopicAdvanceConfig) IsEmpty() bool {
	return a.ContentBasedDeduplication == nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the FIFOTopicAdvanceConfigOrBool
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (t *FIFOTopicAdvanceConfigOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&t.Advanced); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !t.Advanced.IsEmpty() {
		return nil
	}
	if err := value.Decode(&t.Enable); err != nil {
		return errUnmarshalFifoConfig
	}
	return nil
}

// NetworkConfig represents options for network connection to AWS resources within a VPC.
type NetworkConfig struct {
	VPC     vpcConfig                `yaml:"vpc"`
	Connect ServiceConnectBoolOrArgs `yaml:"connect"`
}

// IsEmpty returns empty if the struct has all zero members.
func (c *NetworkConfig) IsEmpty() bool {
	return c.VPC.isEmpty()
}

func (c *NetworkConfig) requiredEnvFeatures() []string {
	if aws.StringValue((*string)(c.VPC.Placement.PlacementString)) == string(PrivateSubnetPlacement) {
		return []string{template.NATFeatureName}
	}
	return nil
}

// ServiceConnectBoolOrArgs represents ECS Service Connect configuration.
type ServiceConnectBoolOrArgs struct {
	EnableServiceConnect *bool
	ServiceConnectArgs
}

// Enabled returns if ServiceConnect is enabled or not.
func (s *ServiceConnectBoolOrArgs) Enabled() bool {
	return aws.BoolValue(s.EnableServiceConnect) || !s.ServiceConnectArgs.isEmpty()
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ServiceConnect
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (s *ServiceConnectBoolOrArgs) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.ServiceConnectArgs); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !s.ServiceConnectArgs.isEmpty() {
		s.EnableServiceConnect = nil
		return nil
	}
	if err := value.Decode(&s.EnableServiceConnect); err != nil {
		return errUnmarshalServiceConnectOpts
	}
	return nil
}

// ServiceConnectArgs includes the advanced configuration for ECS Service Connect.
type ServiceConnectArgs struct {
	Alias *string
}

func (s *ServiceConnectArgs) isEmpty() bool {
	return s.Alias == nil
}

// PlacementArgOrString represents where to place tasks.
type PlacementArgOrString struct {
	*PlacementString
	PlacementArgs
}

// PlacementString represents what types of subnets (public or private subnets) to place tasks.
type PlacementString string

// IsEmpty returns empty if the struct has all zero members.
func (p *PlacementArgOrString) IsEmpty() bool {
	return p.PlacementString == nil && p.PlacementArgs.isEmpty()
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the PlacementArgOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (p *PlacementArgOrString) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&p.PlacementArgs); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !p.PlacementArgs.isEmpty() {
		// Unmarshaled successfully to p.PlacementArgs, unset p.PlacementString, and return.
		p.PlacementString = nil
		return nil
	}
	if err := value.Decode(&p.PlacementString); err != nil {
		return errUnmarshalPlacementOpts
	}
	return nil
}

// PlacementArgs represents where to place tasks.
type PlacementArgs struct {
	Subnets SubnetListOrArgs `yaml:"subnets"`
}

func (p *PlacementArgs) isEmpty() bool {
	return p.Subnets.isEmpty()
}

// SubnetListOrArgs represents what subnets to place tasks. It supports unmarshalling
// yaml which can either be of type SubnetArgs or a list of strings.
type SubnetListOrArgs struct {
	IDs []string
	SubnetArgs
}

func (s *SubnetListOrArgs) isEmpty() bool {
	return len(s.IDs) == 0 && s.SubnetArgs.isEmpty()
}

type dynamicSubnets struct {
	cfg    *SubnetListOrArgs
	client subnetIDsGetter
}

// Load populates the subnet's IDs field if the client is using tags.
func (dyn *dynamicSubnets) load() error {
	if dyn.cfg == nil || dyn.cfg.isEmpty() {
		return nil
	}
	if len(dyn.cfg.IDs) > 0 {
		return nil
	}
	var filters []ec2.Filter
	for k, v := range dyn.cfg.FromTags {
		values := v.StringSlice
		if v.String != nil {
			values = v.ToStringSlice()
		}
		filters = append(filters, ec2.FilterForTags(k, values...))
	}
	ids, err := dyn.client.SubnetIDs(filters...)
	if err != nil {
		return fmt.Errorf("get subnet IDs: %w", err)
	}
	dyn.cfg.IDs = ids
	return nil
}

// Tags represents the aws tags which take string as key and slice of string as values.
type Tags map[string]StringSliceOrString

// SubnetArgs represents what subnets to place tasks.
type SubnetArgs struct {
	FromTags Tags `yaml:"from_tags"`
}

func (s *SubnetArgs) isEmpty() bool {
	return len(s.FromTags) == 0
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the SubnetListOrArgs
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (s *SubnetListOrArgs) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.SubnetArgs); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !s.SubnetArgs.isEmpty() {
		// Unmarshaled successfully to s.SubnetArgs, unset s.Subnets, and return.
		s.IDs = nil
		return nil
	}
	if err := value.Decode(&s.IDs); err != nil {
		return errUnmarshalSubnetsOpts
	}
	return nil
}

// SecurityGroupsIDsOrConfig represents security groups attached to task. It supports unmarshalling
// yaml which can either be of type SecurityGroupsConfig or a list of strings.
type SecurityGroupsIDsOrConfig struct {
	IDs            []StringOrFromCFN
	AdvancedConfig SecurityGroupsConfig
}

func (s *SecurityGroupsIDsOrConfig) isEmpty() bool {
	return len(s.IDs) == 0 && s.AdvancedConfig.isEmpty()
}

// SecurityGroupsConfig represents which security groups are attached to a task
// and if default security group is applied.
type SecurityGroupsConfig struct {
	SecurityGroups []StringOrFromCFN `yaml:"groups"`
	DenyDefault    *bool             `yaml:"deny_default"`
}

func (s *SecurityGroupsConfig) isEmpty() bool {
	return len(s.SecurityGroups) == 0 && s.DenyDefault == nil
}

// UnmarshalYAML overrides the default YAML unmarshalling logic for the SecurityGroupsIDsOrConfig
// struct, allowing it to be unmarshalled into a string slice or a string.
// This method implements the yaml.Unmarshaler (v3) interface.
func (s *SecurityGroupsIDsOrConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.AdvancedConfig); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}

	if !s.AdvancedConfig.isEmpty() {
		// Unmarshalled successfully to s.AdvancedConfig, unset s.IDs, and return.
		s.IDs = nil
		return nil
	}

	if err := value.Decode(&s.IDs); err != nil {
		return errUnmarshalSecurityGroupOpts
	}
	return nil
}

// GetIDs returns security groups from SecurityGroupsIDsOrConfig that are attached to task.
// nil is returned if no security groups are specified.
func (s *SecurityGroupsIDsOrConfig) GetIDs() []StringOrFromCFN {
	if !s.AdvancedConfig.isEmpty() {
		return s.AdvancedConfig.SecurityGroups
	}
	return s.IDs
}

// IsDefaultSecurityGroupDenied returns true if DenyDefault is set to true
// in SecurityGroupsIDsOrConfig.AdvancedConfig. Otherwise, false is returned.
func (s *SecurityGroupsIDsOrConfig) IsDefaultSecurityGroupDenied() bool {
	if !s.AdvancedConfig.isEmpty() {
		return aws.BoolValue(s.AdvancedConfig.DenyDefault)
	}
	return false
}

// vpcConfig represents the security groups and subnets attached to a task.
type vpcConfig struct {
	Placement      PlacementArgOrString      `yaml:"placement"`
	SecurityGroups SecurityGroupsIDsOrConfig `yaml:"security_groups"`
}

func (v *vpcConfig) isEmpty() bool {
	return v.Placement.IsEmpty() && v.SecurityGroups.isEmpty()
}

// PlatformArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type PlatformArgs.
type PlatformArgsOrString struct {
	*PlatformString
	PlatformArgs PlatformArgs
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the PlatformArgsOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (p *PlatformArgsOrString) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&p.PlatformArgs); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !p.PlatformArgs.isEmpty() {
		// Unmarshaled successfully to p.PlatformArgs, unset p.PlatformString, and return.
		p.PlatformString = nil
		return nil
	}
	if err := value.Decode(&p.PlatformString); err != nil {
		return errUnmarshalPlatformOpts
	}
	return nil
}

// OS returns the operating system family.
func (p *PlatformArgsOrString) OS() string {
	if p := aws.StringValue((*string)(p.PlatformString)); p != "" {
		args := strings.Split(p, "/")
		return strings.ToLower(args[0])
	}
	return strings.ToLower(aws.StringValue(p.PlatformArgs.OSFamily))
}

// Arch returns the architecture of PlatformArgsOrString.
func (p *PlatformArgsOrString) Arch() string {
	if p := aws.StringValue((*string)(p.PlatformString)); p != "" {
		args := strings.Split(p, "/")
		return strings.ToLower(args[1])
	}
	return strings.ToLower(aws.StringValue(p.PlatformArgs.Arch))
}

// PlatformArgs represents the specifics of a target OS.
type PlatformArgs struct {
	OSFamily *string `yaml:"osfamily,omitempty"`
	Arch     *string `yaml:"architecture,omitempty"`
}

// PlatformString represents the string format of Platform.
type PlatformString string

// String implements the fmt.Stringer interface.
func (p *PlatformArgs) String() string {
	return fmt.Sprintf("('%s', '%s')", aws.StringValue(p.OSFamily), aws.StringValue(p.Arch))
}

// IsEmpty returns if the platform field is empty.
func (p *PlatformArgsOrString) IsEmpty() bool {
	return p.PlatformString == nil && p.PlatformArgs.isEmpty()
}

func (p *PlatformArgs) isEmpty() bool {
	return p.OSFamily == nil && p.Arch == nil
}

func (p *PlatformArgs) bothSpecified() bool {
	return (p.OSFamily != nil) && (p.Arch != nil)
}

// platformString returns a specified of the format <os>/<arch>.
func platformString(os, arch string) string {
	return fmt.Sprintf("%s/%s", os, arch)
}

// RedirectPlatform returns a platform that's supported for the given manifest type.
func RedirectPlatform(os, arch, wlType string) (platform string, err error) {
	// Return nil if passed the default platform.
	if platformString(os, arch) == defaultPlatform {
		return "", nil
	}
	// Return an error if a platform cannot be redirected.
	if wlType == manifestinfo.RequestDrivenWebServiceType && os == OSWindows {
		return "", ErrAppRunnerInvalidPlatformWindows
	}
	// All architectures default to 'x86_64' (though 'arm64' is now also supported); leave OS as is.
	// If a string is returned, the platform is not the default platform but is supported (except for more obscure platforms).
	return platformString(os, dockerengine.ArchX86), nil
}

func isWindowsPlatform(platform PlatformArgsOrString) bool {
	for _, win := range windowsOSFamilies {
		if platform.OS() == win {
			return true
		}
	}
	return false
}

// IsArmArch returns whether or not the arch is ARM.
func IsArmArch(arch string) bool {
	return strings.ToLower(arch) == ArchARM || strings.ToLower(arch) == ArchARM64
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

func placementStringP(p PlacementString) *PlacementString {
	if p == "" {
		return nil
	}
	placement := p
	return &placement
}

func (cfg PublishConfig) publishedTopics() []Topic {
	if len(cfg.Topics) == 0 {
		return nil
	}
	pubs := make([]Topic, len(cfg.Topics))
	for i, topic := range cfg.Topics {
		if topic.FIFO.IsEnabled() {
			topic.Name = aws.String(aws.StringValue(topic.Name) + ".fifo")
		}
		pubs[i] = topic
	}
	return pubs
}

// ContainerDependencies returns a map of ContainerDependency objects from workload manifest.
func ContainerDependencies(unmarshaledManifest interface{}) map[string]ContainerDependency {
	type containerDependency interface {
		ContainerDependencies() map[string]ContainerDependency
	}
	mf, ok := unmarshaledManifest.(containerDependency)
	if ok {
		return mf.ContainerDependencies()
	}
	return nil
}
