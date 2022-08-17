// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"time"
)

func convertBackendService(service *compose.ServiceConfig, port uint16) (*manifest.BackendServiceConfig, IgnoredKeys, error) {
	image, ignored, err := convertImageConfig(service.Build, service.Labels, service.Image)
	if err != nil {
		return nil, nil, fmt.Errorf("convert image config: %w", err)
	}

	taskCfg, err := convertTaskConfig(service)
	if err != nil {
		return nil, nil, fmt.Errorf("convert task config: %w", err)
	}

	svcCfg := &manifest.BackendServiceConfig{
		ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
			ImageWithOptionalPort: manifest.ImageWithOptionalPort{
				Image: image,
				Port:  &port,
			},
		},
		ImageOverride: manifest.ImageOverride{
			Command: manifest.CommandOverride{
				StringSlice: service.Command,
			},
			EntryPoint: manifest.EntryPointOverride{
				StringSlice: service.Entrypoint,
			},
		},
		TaskConfig: taskCfg,
	}

	if service.HealthCheck != nil {
		svcCfg.ImageConfig.HealthCheck = convertHealthCheckConfig(service.HealthCheck)
	}

	return svcCfg, ignored, nil
}

// convertTaskConfig converts environment variables, env files, and platform strings.
func convertTaskConfig(service *compose.ServiceConfig) (manifest.TaskConfig, error) {
	var envFile *string

	if len(service.EnvFile) == 1 {
		envFile = &service.EnvFile[0]
	} else if len(service.EnvFile) > 1 {
		return manifest.TaskConfig{}, fmt.Errorf("at most one env file is supported, but %d env files "+
			"were attached to this service", len(service.EnvFile))
	}

	taskCfg := manifest.TaskConfig{
		Platform: manifest.PlatformArgsOrString{
			PlatformString: (*manifest.PlatformString)(nilIfEmpty(service.Platform)),
		},
		EnvFile: envFile,
	}

	envVars, err := convertMappingWithEquals(service.Environment)
	if err != nil {
		return manifest.TaskConfig{}, fmt.Errorf("convert environment variables: %w", err)
	}

	if len(envVars) != 0 {
		taskCfg.Variables = envVars
	}

	return taskCfg, nil
}

// convertHealthCheckConfig trivially converts a Compose container health check into its Copilot variant.
func convertHealthCheckConfig(healthcheck *compose.HealthCheckConfig) manifest.ContainerHealthCheck {
	retries := 3
	if healthcheck.Retries != nil {
		retries = int(*healthcheck.Retries)
	}

	cmd := healthcheck.Test
	if healthcheck.Disable {
		cmd = []string{"NONE"}
	}

	return manifest.ContainerHealthCheck{
		Command:     cmd,
		Interval:    (*time.Duration)(healthcheck.Interval),
		Retries:     &retries,
		Timeout:     (*time.Duration)(healthcheck.Timeout),
		StartPeriod: (*time.Duration)(healthcheck.StartPeriod),
	}
}
