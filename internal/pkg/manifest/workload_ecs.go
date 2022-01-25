// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
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

// OverrideRule holds the manifest overriding rule for CloudFormation template.
type OverrideRule struct {
	Path  string    `yaml:"path"`
	Value yaml.Node `yaml:"value"`
}
