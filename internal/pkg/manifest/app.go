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
	// LoadBalancedWebApplication is a web application with a load balancer and Fargate as compute.
	LoadBalancedWebApplication = "Load Balanced Web App"
	// BackendApplication is a service that cannot be accessed from the internet but can be reached from other services.
	BackendApplication = "Backend App"
)

// AppTypes are the supported manifest types.
var AppTypes = []string{
	LoadBalancedWebApplication,
}

// App holds the basic data that every application manifest file needs to have.
type App struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // must be one of the supported manifest types.
}

// AppImage represents the application's container image.
type AppImage struct {
	Build string `yaml:"build"` // Path to the Dockerfile.
}

// AppImageWithPort represents a container image with an exposed port.
type AppImageWithPort struct {
	AppImage `yaml:",inline"`
	Port     uint16 `yaml:"port"`
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

// AppProps contains properties for creating a new manifest.
type AppProps struct {
	AppName    string
	Dockerfile string
}

// UnmarshalApp deserializes the YAML input stream into a application manifest object.
// If an error occurs during deserialization, then returns the error.
// If the application type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalApp(in []byte) (interface{}, error) {
	am := App{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, fmt.Errorf("unmarshal to application manifest: %w", err)
	}

	switch am.Type {
	case LoadBalancedWebApplication:
		m := newDefaultLoadBalancedWebApp()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to load balanced web application: %w", err)
		}
		return m, nil
	case BackendApplication:
		m := newDefaultBackendApp()
		if err := yaml.Unmarshal(in, m); err != nil {
			return nil, fmt.Errorf("unmarshal to backend application: %w", err)
		}
		if m.Image.HealthCheck != nil {
			// Make sure that unset fields in the healthcheck gets a default value.
			m.Image.HealthCheck.applyIfNotSet(newDefaultContainerHealthCheck())
		}
		return m, nil
	default:
		return nil, &ErrInvalidAppManifestType{Type: am.Type}
	}
}

func intp(v int) *int {
	return &v
}

func durationp(v time.Duration) *time.Duration {
	return &v
}
