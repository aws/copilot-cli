// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

const (
	// LoadBalancedWebServiceType is a web service with a load balancer and Fargate as compute.
	LoadBalancedWebServiceType = "Load Balanced Web Service"
	// BackendServiceType is a service that cannot be accessed from the internet but can be reached from other services.
	BackendServiceType = "Backend Service"
)

// ServiceTypes are the supported service manifest types.
var ServiceTypes = []string{
	LoadBalancedWebServiceType,
	BackendServiceType,
}

// Service holds the basic data that every service manifest file needs to have.
type Service struct {
	Name *string `yaml:"name"`
	Type *string `yaml:"type"` // must be one of the supported manifest types.
}

// ServiceImage represents the service's container image.
type ServiceImage struct {
	Build *string `yaml:"build"` // Path to the Dockerfile.
}

// ServiceImageWithPort represents a container image with an exposed port.
type ServiceImageWithPort struct {
	ServiceImage `yaml:",inline"`
	Port         *uint16 `yaml:"port"`
}

// LogConfig holds configuration for Firelens to route your logs.
type LogConfig struct {
	Destination    destinationConfig `yaml:"destination,flow"`
	EnableMetadata *bool             `yaml:"enableMetadata"`
	SecretOptions  map[string]string `yaml:"secretOptions"`
	ConfigFile     *string           `yaml:"configFile"`
	PermissionFile *string           `yaml:"permissionFile"`
}

type destinationConfig struct {
	Name           *string `yaml:"name"`
	IncludePattern *string `yaml:"includePattern"` // can be empty string as a valid value
	ExcludePattern *string `yaml:"excludePattern"`
}

// Sidecar holds configuration for all sidecar containers in a service.
type Sidecar struct {
	Sidecars map[string]*SidecarConfig `yaml:"sidecars"`
}

// SidecarConfig represents the configurable options for setting up a sidecar container.
type SidecarConfig struct {
	Port      *string `yaml:"port"`
	Image     *string `yaml:"image"`
	CredParam *string `yaml:"credentialsParameter"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU       *int              `yaml:"cpu"`
	Memory    *int              `yaml:"memory"`
	Count     *int              `yaml:"count"` // 0 is a valid value, so we want the default value to be nil.
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
}

// ServiceProps contains properties for creating a new service manifest.
type ServiceProps struct {
	Name       string
	Dockerfile string
}

// UnmarshalService deserializes the YAML input stream into a service manifest object.
// If an error occurs during deserialization, then returns the error.
// If the service type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalService(in []byte) (interface{}, error) {
	am := Service{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, fmt.Errorf("unmarshal to service manifest: %w", err)
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
		if m.BackendServiceConfig.Image.HealthCheck != nil {
			// Make sure that unset fields in the healthcheck gets a default value.
			m.BackendServiceConfig.Image.HealthCheck.applyIfNotSet(newDefaultContainerHealthCheck())
		}
		return m, nil
	default:
		return nil, &ErrInvalidSvcManifestType{Type: typeVal}
	}
}

func durationp(v time.Duration) *time.Duration {
	return &v
}

func boolpcopy(v *bool) *bool {
	if v == nil {
		return nil
	}
	return aws.Bool(*v)
}

func stringpcopy(v *string) *string {
	if v == nil {
		return nil
	}
	return aws.String(*v)
}

func intpcopy(v *int) *int {
	if v == nil {
		return nil
	}
	return aws.Int(*v)
}
