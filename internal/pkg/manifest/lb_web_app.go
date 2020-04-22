// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
)

const (
	lbWebAppManifestPath = "applications/lb-web-app/manifest.yml"

	// LogRetentionInDays is the default log retention time in days.
	LogRetentionInDays = 30
)

// LoadBalancedWebApp holds the configuration to build a container image with an exposed port that receives
// requests through a load balancer with AWS Fargate as the compute engine.
type LoadBalancedWebApp struct {
	App                      `yaml:",inline"`
	Image                    AppImageWithPort `yaml:",flow"`
	LoadBalancedWebAppConfig `yaml:",inline"`
	Environments             map[string]LoadBalancedWebAppConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// LoadBalancedWebAppConfig represents a load balanced web application with AWS Fargate as compute.
type LoadBalancedWebAppConfig struct {
	RoutingRule `yaml:"http,flow"`
	TaskConfig  `yaml:",inline"`
	LogsConfig  `yaml:",flow"`
}

// LogsConfig is the configuration to the ECS logs.
type LogsConfig struct {
	LogRetention int `yaml:"logRetention"`
}

// RoutingRule holds the path to route requests to the service.
type RoutingRule struct {
	Path            string `yaml:"path"`
	HealthCheckPath string `yaml:"healthcheck"`
}

func (r RoutingRule) copyAndApply(other RoutingRule) RoutingRule {
	if other.Path != "" {
		r.Path = other.Path
	}
	if other.HealthCheckPath != "" {
		r.HealthCheckPath = other.HealthCheckPath
	}
	return r
}

// LoadBalancedWebAppProps contains properties for creating a new load balanced fargate application manifest.
type LoadBalancedWebAppProps struct {
	*AppProps
	Path string
	Port uint16
}

// NewLoadBalancedWebApp creates a new public load balanced web service, receives all the requests from the load balancer,
// has a single task with minimal CPU and memory thresholds, and sets the default health check path to "/".
func NewLoadBalancedWebApp(input *LoadBalancedWebAppProps) *LoadBalancedWebApp {
	defaultLbManifest := newDefaultLoadBalancedWebApp()
	defaultLbManifest.App = App{
		Name: input.AppName,
		Type: LoadBalancedWebApplication,
	}
	defaultLbManifest.Image = AppImageWithPort{
		AppImage: AppImage{
			Build: input.Dockerfile,
		},
		Port: input.Port,
	}
	defaultLbManifest.LoadBalancedWebAppConfig.RoutingRule.Path = input.Path
	defaultLbManifest.parser = template.New()
	return defaultLbManifest
}

// newDefaultLoadBalancedWebApp returns an empty LoadBalancedWebApp with only the default values set.
func newDefaultLoadBalancedWebApp() *LoadBalancedWebApp {
	return &LoadBalancedWebApp{
		App:   App{},
		Image: AppImageWithPort{},
		LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
			RoutingRule: RoutingRule{
				HealthCheckPath: "/",
			},
			TaskConfig: TaskConfig{
				CPU:    256,
				Memory: 512,
				Count:  intp(1),
			},
			LogsConfig: LogsConfig{
				LogRetention: LogRetentionInDays,
			},
		},
	}
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (a *LoadBalancedWebApp) MarshalBinary() ([]byte, error) {
	content, err := a.parser.Parse(lbWebAppManifestPath, *a)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// DockerfilePath returns the image build path.
func (a *LoadBalancedWebApp) DockerfilePath() string {
	return a.Image.Build
}

// ApplyEnv returns the application manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (a *LoadBalancedWebApp) ApplyEnv(envName string) *LoadBalancedWebApp {
	target, ok := a.Environments[envName]
	if !ok {
		return a
	}

	return &LoadBalancedWebApp{
		App:   a.App,
		Image: a.Image,
		LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
			RoutingRule: a.RoutingRule.copyAndApply(target.RoutingRule),
			TaskConfig:  a.TaskConfig.copyAndApply(target.TaskConfig),
			LogsConfig: LogsConfig{
				LogRetention: a.LogRetention,
			},
		},
	}
}
