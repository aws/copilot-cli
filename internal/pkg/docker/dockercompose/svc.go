package dockercompose

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
	"time"
)

func convertBackendService(service *types.ServiceConfig, port uint16) (*manifest.BackendServiceConfig, IgnoredKeys, error) {
	image, ignored, err := convertImageConfig(service.Build, service.Labels, service.Image)
	if err != nil {
		return nil, ignored, fmt.Errorf("convert image config: %w", err)
	}

	taskCfg, err := convertTaskConfig(service)
	if err != nil {
		return nil, ignored, fmt.Errorf("convert task config: %w", err)
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
		for ext := range service.HealthCheck.Extensions {
			ignored = append(ignored, "healthcheck."+ext)
		}
	}

	for ext := range service.Extensions {
		ignored = append(ignored, ext)
	}

	return svcCfg, ignored, nil
}

// convertTaskConfig converts environment variables, env files, and platform strings.
func convertTaskConfig(service *types.ServiceConfig) (manifest.TaskConfig, error) {
	var envFile *string

	if service.EnvFile != nil {
		if len(service.EnvFile) == 1 {
			envFile = &service.EnvFile[0]
		} else if len(service.EnvFile) > 1 {
			return manifest.TaskConfig{}, fmt.Errorf("at most one env file is supported, but %d env files "+
				"were attached to this service", len(service.EnvFile))
		}
	}

	envVars, err := convertMappingWithEquals(service.Environment)
	if err != nil {
		return manifest.TaskConfig{}, fmt.Errorf("convert environment variables: %w", err)
	}

	return manifest.TaskConfig{
		Platform: manifest.PlatformArgsOrString{
			PlatformString: (*manifest.PlatformString)(nilIfEmpty(service.Platform)),
		},
		Variables: envVars,
		EnvFile:   envFile,
	}, nil
}

// convertHealthCheckConfig trivially converts a Compose container health check into its Copilot variant.
func convertHealthCheckConfig(healthcheck *types.HealthCheckConfig) manifest.ContainerHealthCheck {
	return manifest.ContainerHealthCheck{
		Command:     healthcheck.Test,
		Interval:    (*time.Duration)(healthcheck.Interval),
		Retries:     aws.Int(int(*healthcheck.Retries)),
		Timeout:     (*time.Duration)(healthcheck.Timeout),
		StartPeriod: (*time.Duration)(healthcheck.StartPeriod),
	}
}
