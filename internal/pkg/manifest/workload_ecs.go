// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import "github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

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
