// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"gopkg.in/yaml.v3"
)

// Defaults for Firelens configuration.
const (
	firelensContainerName = "firelens_log_router"
	defaultFluentbitImage = "public.ecr.aws/aws-observability/aws-for-fluent-bit:latest"
)

// Platform related settings.
const (
	OSLinux                 = dockerengine.OSLinux
	OSWindows               = dockerengine.OSWindows
	OSWindowsServer2019Core = "windows_server_2019_core"
	OSWindowsServer2019Full = "windows_server_2019_full"

	ArchAMD64 = dockerengine.ArchAMD64
	ArchX86   = dockerengine.ArchX86
	ArchARM   = dockerengine.ArchARM
	ArchARM64 = dockerengine.ArchARM64

	// Minimum CPU and mem values required for Windows-based tasks.
	MinWindowsTaskCPU    = 1024
	MinWindowsTaskMemory = 2048
)

// Platform related settings.
var (
	defaultPlatform     = platformString(OSLinux, ArchAMD64)
	windowsOSFamilies   = []string{OSWindows, OSWindowsServer2019Core, OSWindowsServer2019Full}
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
	}
)

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU            *int                 `yaml:"cpu"`
	Memory         *int                 `yaml:"memory"`
	Platform       PlatformArgsOrString `yaml:"platform,omitempty"`
	Count          Count                `yaml:"count"`
	ExecuteCommand ExecuteCommand       `yaml:"exec"`
	Variables      map[string]string    `yaml:"variables"`
	EnvFile        *string              `yaml:"env_file"`
	Secrets        map[string]string    `yaml:"secrets"`
	Storage        Storage              `yaml:"storage"`
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
func (t *TaskConfig) IsWindows() bool {
	return isWindowsPlatform(t.Platform)
}

// IsARM returns whether or not the service is building with an ARM Arch.
func (t *TaskConfig) IsARM() bool {
	return IsArmArch(t.Platform.Arch())
}

// Logging holds configuration for Firelens to route your logs.
type Logging struct {
	Retention      *int              `yaml:"retention"`
	Image          *string           `yaml:"image"`
	Destination    map[string]string `yaml:"destination,flow"`
	EnableMetadata *bool             `yaml:"enableMetadata"`
	SecretOptions  map[string]string `yaml:"secretOptions"`
	ConfigFile     *string           `yaml:"configFilePath"`
	Variables      map[string]string `yaml:"variables"`
	Secrets        map[string]string `yaml:"secrets"`
}

// IsEmpty returns empty if the struct has all zero members.
func (lc *Logging) IsEmpty() bool {
	return lc.Image == nil && lc.Destination == nil && lc.EnableMetadata == nil &&
		lc.SecretOptions == nil && lc.ConfigFile == nil && lc.Variables == nil && lc.Secrets == nil
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
	DependsOn     DependsOn            `yaml:"depends_on"`
	HealthCheck   ContainerHealthCheck `yaml:"healthcheck"`
	ImageOverride `yaml:",inline"`
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
