// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/templates"
)

// LoadBalancedFargateManifest holds the fields needed to represent a load balanced Fargate service.
type LoadBalancedFargateManifest struct {
	AppManifest `yaml:",inline"`
	Image       LBFargateImage `yaml:",flow"`
	CPU         int            `yaml:"cpu"`     // Number of CPU units used by the task.
	Memory      int            `yaml:"memory"`  // Amount of memory in MiBused by the task.
	Logging     bool           `yaml:"logging"` // True means that a log group will be created.
	Public      bool           `yaml:"public"`  // True means a public endpoint will be created.
	Stages      []AppStage     `yaml:",flow"`   // Deployment stages for this application.
}

type LBFargateImage struct {
	AppImage `yaml:",inline"`
	Port     int `yaml:"port"` // Port exposed in the container.
}

// NewLoadBalancedFargateManifest creates a new public load balanced Fargate service with minimal compute settings.
func NewLoadBalancedFargateManifest(appName string, dockerfile string) *LoadBalancedFargateManifest {
	return &LoadBalancedFargateManifest{
		AppManifest: AppManifest{Name: appName, Type: LoadBalancedWebApplication},
		Image: LBFargateImage{
			AppImage: AppImage{
				Build: dockerfile,
			},
			Port: 8080,
		},
		CPU:     256,
		Memory:  512,
		Logging: true,
		Public:  true,
		Stages:  []AppStage{},
	}
}

// Marshal serializes the manifest object into a YAML document.
func (m *LoadBalancedFargateManifest) Marshal() ([]byte, error) {
	box := templates.Box()
	content, err := box.FindString("lb-fargate-service/manifest.yml")
	if err != nil {
		return nil, err
	}
	tpl, err := template.New("template").Parse(content)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, *m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CFNTemplate serializes the manifest object into a CloudFormation template.
func (m *LoadBalancedFargateManifest) CFNTemplate() (string, error) {
	return "", nil
}
