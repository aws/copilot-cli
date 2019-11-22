// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"gopkg.in/yaml.v3"
)

const (
	// LoadBalancedWebApplication is a web application with a load balancer and Fargate as compute.
	LoadBalancedWebApplication = "Load Balanced Web App"
)

// AppTypes are the supported manifest types.
var AppTypes = []string{
	LoadBalancedWebApplication,
}

// AppManifest holds the basic data that every manifest file need to have.
type AppManifest struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // must be one of the supported manifest types.
}

// AppName returns the name of the application
func (a *AppManifest) AppName() string {
	return a.Name
}

// AppImage represents the application's container image.
type AppImage struct {
	Build string `yaml:"build"` // Path to the Dockerfile.
}

// CreateApp returns a manifest object based on the application's type.
// If the application type is invalid, then returns an ErrInvalidManifestType.
func CreateApp(appName, appType, dockerfile string) (archer.Manifest, error) {
	switch appType {
	case LoadBalancedWebApplication:
		return NewLoadBalancedFargateManifest(appName, dockerfile), nil
	default:
		return nil, &ErrInvalidAppManifestType{Type: appType}
	}
}

// UnmarshalApp deserializes the YAML input stream into a manifest object.
// If an error occurs during deserialization, then returns the error.
// If the application type in the manifest is invalid, then returns an ErrInvalidManifestType.
func UnmarshalApp(in []byte) (archer.Manifest, error) {
	am := AppManifest{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, &ErrUnmarshalAppManifest{parent: err}
	}

	switch am.Type {
	case LoadBalancedWebApplication:
		m := LBFargateManifest{}
		if err := yaml.Unmarshal(in, &m); err != nil {
			return nil, &ErrUnmarshalLBFargateManifest{parent: err}
		}
		return &m, nil
	default:
		return nil, &ErrInvalidAppManifestType{Type: am.Type}
	}
}
