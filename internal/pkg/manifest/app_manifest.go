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

// AppStage represents configuration for each deployment stage of an application.
type AppStage struct {
	EnvName      string `yaml:"env"`
	DesiredCount int    `yaml:"desiredCount"`
}

// CreateApp returns a manifest object based on the application's type.
// If the application type is invalid, then returns an ErrInvalidManifestType.
func CreateApp(appName, appType string) (archer.Manifest, error) {
	switch appType {
	case LoadBalancedWebApplication:
		return NewLoadBalancedFargateManifest(appName), nil
	default:
		return nil, &ErrInvalidManifestType{Type: appType}
	}
}

// Unmarshal deserializes the YAML input stream into a manifest object.
// If an error occurs during deserialization, then returns the error.
// If the application type in the manifest is invalid, then returns an ErrInvalidManifestType.
func Unmarshal(in []byte) (archer.Manifest, error) {
	am := AppManifest{}
	if err := yaml.Unmarshal(in, &am); err != nil {
		return nil, err
	}

	switch am.Type {
	case LoadBalancedWebApplication:
		m := LoadBalancedFargateManifest{}
		if err := yaml.Unmarshal(in, &m); err != nil {
			return nil, err
		}
		return &m, nil
	default:
		return nil, &ErrInvalidManifestType{Type: am.Type}
	}
}
