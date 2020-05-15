// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"fmt"
	"time"

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
	Name string `yaml:"name"`
	Type string `yaml:"type"` // must be one of the supported manifest types.
}

// ServiceImage represents the service's container image.
type ServiceImage struct {
	Build string `yaml:"build"` // Path to the Dockerfile.
}

// ServiceImageWithPort represents a container image with an exposed port.
type ServiceImageWithPort struct {
	ServiceImage `yaml:",inline"`
	Port         uint16 `yaml:"port"`
}

// TaskConfig represents the resource boundaries and environment variables for the containers in the task.
type TaskConfig struct {
	CPU       int               `yaml:"cpu"`
	Memory    int               `yaml:"memory"`
	Count     *int              `yaml:"count"` // 0 is a valid value, so we want the default value to be nil.
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
}

func (tc TaskConfig) copyAndApply(other TaskConfig) TaskConfig {
	override := tc.deepcopy()
	if other.CPU != 0 {
		override.CPU = other.CPU
	}
	if other.Memory != 0 {
		override.Memory = other.Memory
	}
	if other.Count != nil {
		override.Count = intp(*other.Count)
	}
	for k, v := range other.Variables {
		override.Variables[k] = v
	}
	for k, v := range other.Secrets {
		override.Secrets[k] = v
	}
	return override
}

func (tc TaskConfig) deepcopy() TaskConfig {
	vars := make(map[string]string, len(tc.Variables))
	for k, v := range tc.Variables {
		vars[k] = v
	}
	secrets := make(map[string]string, len(tc.Secrets))
	for k, v := range tc.Secrets {
		secrets[k] = v
	}
	return TaskConfig{
		CPU:       tc.CPU,
		Memory:    tc.Memory,
		Count:     intp(*tc.Count),
		Variables: vars,
		Secrets:   secrets,
	}
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

	switch am.Type {
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
		if m.Image.HealthCheck != nil {
			// Make sure that unset fields in the healthcheck gets a default value.
			m.Image.HealthCheck.applyIfNotSet(newDefaultContainerHealthCheck())
		}
		return m, nil
	default:
		return nil, &ErrInvalidSvcManifestType{Type: am.Type}
	}
}

func intp(v int) *int {
	return &v
}

func durationp(v time.Duration) *time.Duration {
	return &v
}
