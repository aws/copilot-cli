// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"gopkg.in/yaml.v3"
)

// Defaults for Firelens configuration.
const (
	FirelensContainerName = "firelens_log_router"
	defaultFluentbitImage = "public.ecr.aws/aws-observability/aws-for-fluent-bit:stable"
)

// Platform related settings.
const (
	OSLinux                 = dockerengine.OSLinux
	OSWindows               = dockerengine.OSWindows
	OSWindowsServer2019Core = "windows_server_2019_core"
	OSWindowsServer2019Full = "windows_server_2019_full"
	OSWindowsServer2022Core = "windows_server_2022_core"
	OSWindowsServer2022Full = "windows_server_2022_full"

	ArchAMD64 = dockerengine.ArchAMD64
	ArchX86   = dockerengine.ArchX86
	ArchARM   = dockerengine.ArchARM
	ArchARM64 = dockerengine.ArchARM64

	// Minimum CPU and mem values required for Windows-based tasks.
	MinWindowsTaskCPU    = 1024
	MinWindowsTaskMemory = 2048

	// deployment strategies
	ECSDefaultRollingUpdateStrategy  = "default"
	ECSRecreateRollingUpdateStrategy = "recreate"
)

// Platform related settings.
var (
	defaultPlatform     = platformString(OSLinux, ArchAMD64)
	windowsOSFamilies   = []string{OSWindows, OSWindowsServer2019Core, OSWindowsServer2019Full, OSWindowsServer2022Core, OSWindowsServer2022Full}
	validShortPlatforms = []string{ // All of the os/arch combinations that the PlatformString field may accept.
		dockerengine.PlatformString(OSLinux, ArchAMD64),
		dockerengine.PlatformString(OSLinux, ArchX86),
		dockerengine.PlatformString(OSLinux, ArchARM),
		dockerengine.PlatformString(OSLinux, ArchARM64),
		dockerengine.PlatformString(OSWindows, ArchAMD64),
		dockerengine.PlatformString(OSWindows, ArchX86),
	}
	validAdvancedPlatforms = []PlatformArgs{ // All of the OsFamily/Arch combinations that the PlatformArgs field may accept.
		{OSFamily: aws.String(OSLinux), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSLinux), Arch: aws.String(ArchAMD64)},
		{OSFamily: aws.String(OSLinux), Arch: aws.String(ArchARM)},
		{OSFamily: aws.String(OSLinux), Arch: aws.String(ArchARM64)},
		{OSFamily: aws.String(OSWindows), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSWindows), Arch: aws.String(ArchAMD64)},
		{OSFamily: aws.String(OSWindowsServer2019Core), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSWindowsServer2019Core), Arch: aws.String(ArchAMD64)},
		{OSFamily: aws.String(OSWindowsServer2019Full), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSWindowsServer2019Full), Arch: aws.String(ArchAMD64)},
		{OSFamily: aws.String(OSWindowsServer2022Core), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSWindowsServer2022Core), Arch: aws.String(ArchAMD64)},
		{OSFamily: aws.String(OSWindowsServer2022Full), Arch: aws.String(ArchX86)},
		{OSFamily: aws.String(OSWindowsServer2022Full), Arch: aws.String(ArchAMD64)},
	}
)

// ImageWithHealthcheck represents a container image with health check.
type ImageWithHealthcheck struct {
	Image       Image                `yaml:",inline"`
	HealthCheck ContainerHealthCheck `yaml:"healthcheck"`
}

// ImageWithPortAndHealthcheck represents a container image with an exposed port and health check.
type ImageWithPortAndHealthcheck struct {
	ImageWithPort `yaml:",inline"`
	HealthCheck   ContainerHealthCheck `yaml:"healthcheck"`
}

// AlarmArgs represents specs of CloudWatch alarms for deployment rollbacks.
type AlarmArgs struct {
	CPUUtilization    *float64 `yaml:"cpu_utilization"`
	MemoryUtilization *float64 `yaml:"memory_utilization"`
}

// WorkerAlarmArgs represents specs of CloudWatch alarms for Worker Service deployment rollbacks.
type WorkerAlarmArgs struct {
	AlarmArgs       `yaml:",inline"`
	MessagesDelayed *int `yaml:"messages_delayed"`
}

// DeploymentControllerConfig represents deployment strategies for a service.
type DeploymentControllerConfig struct {
	Rolling *string `yaml:"rolling"`
}

// DeploymentConfig represents the deployment config for an ECS service.
type DeploymentConfig struct {
	DeploymentControllerConfig `yaml:",inline"`
	RollbackAlarms             Union[[]string, AlarmArgs] `yaml:"rollback_alarms"`
}

// WorkerDeploymentConfig represents the deployment strategies for a worker service.
type WorkerDeploymentConfig struct {
	DeploymentControllerConfig `yaml:",inline"`
	WorkerRollbackAlarms       Union[[]string, WorkerAlarmArgs] `yaml:"rollback_alarms"`
}

func (d *DeploymentConfig) isEmpty() bool {
	return d == nil || (d.DeploymentControllerConfig.isEmpty() && d.RollbackAlarms.IsZero())
}

func (d *DeploymentControllerConfig) isEmpty() bool {
	return d.Rolling == nil
}

func (w *WorkerDeploymentConfig) isEmpty() bool {
	return w == nil || (w.DeploymentControllerConfig.Rolling == nil && w.WorkerRollbackAlarms.IsZero())
}

// ExposedPort will hold the port mapping configuration.
type ExposedPort struct {
	ContainerName        string // The name of the container that exposes this port.
	Port                 uint16 // The port number.
	Protocol             string // Either "tcp" or "udp", empty means the default value that the underlying service provides.
	isDefinedByContainer bool   // Defines if the container port is exposed from "image.port" or "sidecar.port". defaults to false.
}

// ImageWithHealthcheckAndOptionalPort represents a container image with an optional exposed port and health check.
type ImageWithHealthcheckAndOptionalPort struct {
	ImageWithOptionalPort `yaml:",inline"`
	HealthCheck           ContainerHealthCheck `yaml:"healthcheck"`
}

// ImageWithOptionalPort represents a container image with an optional exposed port.
type ImageWithOptionalPort struct {
	Image Image   `yaml:",inline"`
	Port  *uint16 `yaml:"port"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU            *int                 `yaml:"cpu"`
	Memory         *int                 `yaml:"memory"`
	Platform       PlatformArgsOrString `yaml:"platform,omitempty"`
	Count          Count                `yaml:"count"`
	ExecuteCommand ExecuteCommand       `yaml:"exec"`
	Variables      map[string]Variable  `yaml:"variables"`
	EnvFile        *string              `yaml:"env_file"`
	Secrets        map[string]Secret    `yaml:"secrets"`
	Storage        Storage              `yaml:"storage"`
}

// Variable represents an identifier for the value of an environment variable.
type Variable struct {
	StringOrFromCFN
}

// UnmarshalYAML implements the yaml.Unmarshaler (v3) interface to override the default YAML unmarshalling logic.
func (v *Variable) UnmarshalYAML(value *yaml.Node) error {
	if err := v.StringOrFromCFN.UnmarshalYAML(value); err != nil {
		return fmt.Errorf(`unmarshal "variables": %w`, err)
	}
	return nil
}

// RequiresImport returns true if the value is imported from an environment.
func (v *Variable) RequiresImport() bool {
	return !v.FromCFN.isEmpty()
}

// Value returns the value, whether it is used for import or not.
func (v *Variable) Value() string {
	if v.RequiresImport() {
		return aws.StringValue(v.FromCFN.Name)
	}
	return aws.StringValue(v.Plain)
}

// ContainerPlatform returns the platform for the service.
func (t *TaskConfig) ContainerPlatform() string {
	if t.Platform.IsEmpty() {
		return ""
	}
	if t.IsWindows() {
		return platformString(OSWindows, t.Platform.Arch())
	}
	return platformString(t.Platform.OS(), t.Platform.Arch())
}

// IsWindows returns whether or not the service is building with a Windows OS.
func (t TaskConfig) IsWindows() bool {
	return isWindowsPlatform(t.Platform)
}

// IsARM returns whether or not the service is building with an ARM Arch.
func (t TaskConfig) IsARM() bool {
	return IsArmArch(t.Platform.Arch())
}

// Secret represents an identifier for sensitive data stored in either SSM or SecretsManager.
type Secret struct {
	from               StringOrFromCFN      // SSM Parameter name or ARN to a secret or secret ARN imported from another CloudFormation stack.
	fromSecretsManager secretsManagerSecret // Conveniently fetch from a secretsmanager secret name instead of ARN.
}

// UnmarshalYAML implements the yaml.Unmarshaler (v3) interface to override the default YAML unmarshaling logic.
func (s *Secret) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.fromSecretsManager); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}
	if !s.fromSecretsManager.IsEmpty() { // Successfully unmarshaled to a secretsmanager name.
		return nil
	}
	if err := value.Decode(&s.from); err != nil { // Otherwise, try decoding the simple form.
		return errors.New(`cannot marshal "secret" field to a string or "secretsmanager" object`)
	}
	return nil
}

// IsSecretsManagerName returns true if the secret refers to the name of a secret stored in SecretsManager.
func (s *Secret) IsSecretsManagerName() bool {
	return !s.fromSecretsManager.IsEmpty()
}

// RequiresImport returns true if the SSM parameter name or secret ARN value is imported from CloudFormation stack.
func (s *Secret) RequiresImport() bool {
	return !s.from.FromCFN.isEmpty()
}

// Value returns the secret value provided by clients.
func (s *Secret) Value() string {
	if !s.fromSecretsManager.IsEmpty() {
		return aws.StringValue(s.fromSecretsManager.Name)
	} else if s.RequiresImport() {
		return aws.StringValue(s.from.FromCFN.Name)
	}
	return aws.StringValue(s.from.Plain)
}

// secretsManagerSecret represents the name of a secret stored in SecretsManager.
type secretsManagerSecret struct {
	Name *string `yaml:"secretsmanager"`
}

// IsEmpty returns true if all the fields in secretsManagerSecret have the zero value.
func (s secretsManagerSecret) IsEmpty() bool {
	return s.Name == nil
}

// Logging holds configuration for Firelens to route your logs.
type Logging struct {
	Retention      *int                `yaml:"retention"`
	Image          *string             `yaml:"image"`
	Destination    map[string]string   `yaml:"destination,flow"`
	EnableMetadata *bool               `yaml:"enableMetadata"`
	SecretOptions  map[string]Secret   `yaml:"secretOptions"`
	ConfigFile     *string             `yaml:"configFilePath"`
	Variables      map[string]Variable `yaml:"variables"`
	Secrets        map[string]Secret   `yaml:"secrets"`
	EnvFile        *string             `yaml:"env_file"`
}

// IsEmpty returns empty if the struct has all zero members.
func (lc *Logging) IsEmpty() bool {
	return lc.Image == nil && lc.Destination == nil && lc.EnableMetadata == nil && lc.SecretOptions == nil &&
		lc.ConfigFile == nil && lc.Variables == nil && lc.Secrets == nil && lc.EnvFile == nil
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
	Port          *string                              `yaml:"port"`
	Image         Union[*string, ImageLocationOrBuild] `yaml:"image"`
	Essential     *bool                                `yaml:"essential"`
	CredsParam    *string                              `yaml:"credentialsParameter"`
	Variables     map[string]Variable                  `yaml:"variables"`
	EnvFile       *string                              `yaml:"env_file"`
	Secrets       map[string]Secret                    `yaml:"secrets"`
	MountPoints   []SidecarMountPoint                  `yaml:"mount_points"`
	DockerLabels  map[string]string                    `yaml:"labels"`
	DependsOn     DependsOn                            `yaml:"depends_on"`
	HealthCheck   ContainerHealthCheck                 `yaml:"healthcheck"`
	ImageOverride `yaml:",inline"`
}

// ImageURI returns the location of the image if one is set.
// If the image needs to be build, return "" and false.
func (cfg *SidecarConfig) ImageURI() (string, bool) {
	if cfg.Image.Basic != nil {
		return aws.StringValue(cfg.Image.Basic), true
	}
	if cfg.Image.Advanced.Location != nil {
		return aws.StringValue(cfg.Image.Advanced.Location), true
	}
	return "", false
}

// OverrideRule holds the manifest overriding rule for CloudFormation template.
type OverrideRule struct {
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}

// ExecuteCommand is a custom type which supports unmarshaling yaml which
// can either be of type bool or type ExecuteCommandConfig.
type ExecuteCommand struct {
	Enable *bool
	Config ExecuteCommandConfig
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ExecuteCommand
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (e *ExecuteCommand) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&e.Config); err != nil {
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

	if err := value.Decode(&e.Enable); err != nil {
		return errUnmarshalExec
	}
	return nil
}

// ExecuteCommandConfig represents the configuration for ECS Execute Command.
type ExecuteCommandConfig struct {
	Enable *bool `yaml:"enable"`
}

// IsEmpty returns whether ExecuteCommandConfig is empty.
func (e ExecuteCommandConfig) IsEmpty() bool {
	return e.Enable == nil
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

func envFiles(name *string, tc TaskConfig, lc Logging, sc map[string]*SidecarConfig) map[string]string {
	envFiles := make(map[string]string)
	// Grab the workload container's env file, if present.
	envFiles[aws.StringValue(name)] = aws.StringValue(tc.EnvFile)
	// Grab sidecar env files, if present.
	for sidecarName, sidecar := range sc {
		envFiles[sidecarName] = aws.StringValue(sidecar.EnvFile)
	}
	// If the Firelens Sidecar Pattern has an env file specified, get it as well.
	envFiles[FirelensContainerName] = aws.StringValue(lc.EnvFile)
	return envFiles
}

func buildArgs(contextDir string, buildArgs map[string]*DockerBuildArgs, sc map[string]*SidecarConfig) (map[string]*DockerBuildArgs, error) {
	for name, config := range sc {
		if _, ok := config.ImageURI(); !ok {
			buildArgs[name] = config.Image.Advanced.BuildConfig(contextDir)
		}
	}
	return buildArgs, nil
}

// ContainerDependency represents order of container startup and shutdown.
// Also indicates if a container is marked as essential or not.
type ContainerDependency struct {
	IsEssential bool
	DependsOn   DependsOn
}

func containerDependencies(name string, img Image, lc Logging, sc map[string]*SidecarConfig) map[string]ContainerDependency {
	containerDependencies := make(map[string]ContainerDependency)
	containerDependencies[name] = ContainerDependency{
		DependsOn:   img.DependsOn,
		IsEssential: true,
	}
	if !lc.IsEmpty() {
		containerDependencies[FirelensContainerName] = ContainerDependency{}
	}
	for name, config := range sc {
		containerDependencies[name] = ContainerDependency{
			DependsOn:   config.DependsOn,
			IsEssential: config.Essential == nil || aws.BoolValue(config.Essential),
		}
	}
	return containerDependencies
}
