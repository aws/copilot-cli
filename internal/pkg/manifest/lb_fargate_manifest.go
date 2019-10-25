// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/templates"
)

// LBFargateManifest represents a load balanced web application with AWS Fargate as compute.
type LBFargateManifest struct {
	AppManifest     `yaml:",inline"`
	Image           LBFargateImage `yaml:",flow"`
	LBFargateConfig `yaml:",inline"`
	Environments    map[string]LBFargateConfig `yaml:",flow"` // Fields to override per environment.
}

// LBFargateImage represents a container image with its exposed port to receive requests.
type LBFargateImage struct {
	AppImage `yaml:",inline"`
	Port     int `yaml:"port"`
}

// LBFargateConfig are the essential configuration that
type LBFargateConfig struct {
	RoutingRule `yaml:"http,flow"`
	TaskConfig  `yaml:",inline"`
	Public      bool               `yaml:"public"`
	Scaling     *AutoScalingConfig `yaml:",flow"`
}

type TaskConfig struct {
	CPU       int               `yaml:"cpu"`
	Memory    int               `yaml:"memory"`
	Count     int               `yaml:"count"`
	Variables map[string]string `yaml:"variables"`
	Secrets   map[string]string `yaml:"secrets"`
}

type RoutingRule struct {
	Path string `yaml:"path"`
}

type AutoScalingConfig struct {
	MinCount int `yaml:"minCount"`
	MaxCount int `yaml:"maxCount"`

	TargetCPU    float64 `yaml:"targetCPU"`
	TargetMemory float64 `yaml:"targetMemory"`
}

// NewLoadBalancedFargateManifest creates a new public load balanced Fargate service with minimal compute settings.
func NewLoadBalancedFargateManifest(appName string, dockerfile string) *LBFargateManifest {
	return &LBFargateManifest{
		AppManifest: AppManifest{
			Name: appName,
			Type: LoadBalancedWebApplication,
		},
		Image: LBFargateImage{
			AppImage: AppImage{
				Build: dockerfile,
			},
			Port: 80,
		},
		LBFargateConfig: LBFargateConfig{
			RoutingRule: RoutingRule{
				Path: "*",
			},
			TaskConfig: TaskConfig{
				CPU:    256,
				Memory: 512,
				Count:  1,
			},
		},
	}
}

// Marshal serializes the manifest object into a YAML document.
func (m *LBFargateManifest) Marshal() ([]byte, error) {
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
func (m *LBFargateManifest) CFNTemplate() (string, error) {
	return "", nil
}
