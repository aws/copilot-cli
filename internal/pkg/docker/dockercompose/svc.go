// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/spf13/afero"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ConvertedService struct {
	LbSvc      *manifest.LoadBalancedWebService
	BackendSvc *manifest.BackendService
}

func convertService(service *compose.ServiceConfig, workingDir string) (*ConvertedService, IgnoredKeys, error) {
	image, ignored, err := convertImageConfig(service.Build, service.Labels, service.Image)
	if err != nil {
		return nil, nil, err
	}

	taskCfg, err := convertTaskConfig(service)
	if err != nil {
		return nil, nil, err
	}

	imgOverride := manifest.ImageOverride{
		Command: manifest.CommandOverride{
			StringSlice: service.Command,
		},
		EntryPoint: manifest.EntryPointOverride{
			StringSlice: service.Entrypoint,
		},
	}

	var hc manifest.ContainerHealthCheck
	if service.HealthCheck != nil {
		hc = convertHealthCheckConfig(service.HealthCheck)
	}

	exposed, err := findExposedPort(service, workingDir)
	if err != nil {
		return nil, nil, err
	}

	if exposed != nil && exposed.public {
		lbws := manifest.LoadBalancedWebService{}
		lbws.Workload = manifest.Workload{
			Name: &service.Name,
			Type: aws.String(manifest.LoadBalancedWebServiceType),
		}
		lbws.LoadBalancedWebServiceConfig = manifest.LoadBalancedWebServiceConfig{
			ImageConfig: manifest.ImageWithPortAndHealthcheck{
				ImageWithPort: manifest.ImageWithPort{
					Image: image,
					Port:  &exposed.port,
				},
				HealthCheck: hc,
			},
			ImageOverride: imgOverride,
			TaskConfig:    taskCfg,
		}
		return &ConvertedService{LbSvc: &lbws}, ignored, nil
	}

	var port *uint16
	if exposed != nil {
		port = &exposed.port
	}

	bs := manifest.BackendService{}
	bs.Workload = manifest.Workload{
		Name: &service.Name,
		Type: aws.String(manifest.BackendServiceType),
	}
	bs.BackendServiceConfig = manifest.BackendServiceConfig{
		ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
			ImageWithOptionalPort: manifest.ImageWithOptionalPort{
				Image: image,
				Port:  port,
			},
			HealthCheck: hc,
		},
		ImageOverride: imgOverride,
		TaskConfig:    taskCfg,
	}
	return &ConvertedService{BackendSvc: &bs}, ignored, nil
}

type exposedPort struct {
	port   uint16
	public bool
}

// findExposedPort attempts to detect a singular exposed port in the given service and determines if it is publicly exposed.
func findExposedPort(service *compose.ServiceConfig, workingDir string) (*exposedPort, error) {
	if len(service.Ports) == 1 {
		binding := service.Ports[0]
		port := uint16(binding.Target)

		if binding.Published != "" && strconv.Itoa(int(port)) != binding.Published {
			return nil, fmt.Errorf("cannot publish the container port %v under a different public port %v in Copilot", port, binding.Published)
		}

		return &exposedPort{
			port:   port,
			public: true,
		}, nil
	} else if len(service.Ports) > 1 {
		return nil, fmt.Errorf("cannot expose more than one public port in Copilot, but %v ports are exposed publicly: %v",
			len(service.Ports), service.Ports)
	}

	if len(service.Expose) == 1 {
		port, err := strconv.Atoi(service.Expose[0])
		if err != nil {
			return nil, fmt.Errorf("could not parse exposed port: %w", err)
		}

		return &exposedPort{
			port:   uint16(port),
			public: false,
		}, nil
	} else if len(service.Expose) > 1 {
		return nil, fmt.Errorf("cannot expose more than one port in Copilot, but %v ports are exposed: %s",
			len(service.Expose), strings.Join(service.Expose, ", "))
	}

	if service.Image != "" || service.Build == nil {
		// No dockerfile to parse, don't infer any ports.
		return nil, nil
	}

	if service.Build.Context == "" {
		return nil, errors.New("service is missing an image location or Dockerfile path")
	}

	dockerfilePath := service.Build.Dockerfile
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	dockerfilePath = filepath.Join(workingDir, service.Build.Context, dockerfilePath)

	df := dockerfile.New(&afero.Afero{Fs: afero.NewOsFs()}, dockerfilePath)
	ports, err := df.GetExposedPorts()
	var exposeErr dockerfile.ErrNoExpose
	if err != nil && !errors.As(err, &exposeErr) {
		return nil, fmt.Errorf("parse dockerfile for exposed ports: %w", err)
	}

	if len(ports) == 0 {
		// No exposed ports
		return nil, nil
	}

	return &exposedPort{
		// matches "svc init" behavior
		port:   ports[0].Port,
		public: false,
	}, nil
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
		Count: manifest.Count{
			Value: aws.Int(1),
		},
		CPU:    aws.Int(256),
		Memory: aws.Int(512),
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
