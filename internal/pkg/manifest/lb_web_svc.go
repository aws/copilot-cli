// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
)

const (
	lbWebSvcManifestPath = "services/lb-web/manifest.yml"

	// LogRetentionInDays is the default log retention time in days.
	LogRetentionInDays = 30
)

// LoadBalancedWebSvc holds the configuration to build a container image with an exposed port that receives
// requests through a load balancer with AWS Fargate as the compute engine.
type LoadBalancedWebSvc struct {
	Svc                      `yaml:",inline"`
	Image                    SvcImageWithPort `yaml:",flow"`
	LoadBalancedWebSvcConfig `yaml:",inline"`
	Environments             map[string]LoadBalancedWebSvcConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// LoadBalancedWebSvcConfig represents a load balanced web service with AWS Fargate as compute.
type LoadBalancedWebSvcConfig struct {
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

// LoadBalancedWebSvcProps contains properties for creating a new load balanced fargate service manifest.
type LoadBalancedWebSvcProps struct {
	*SvcProps
	Path string
	Port uint16
}

// NewLoadBalancedWebSvc creates a new public load balanced web service, receives all the requests from the load balancer,
// has a single task with minimal CPU and memory thresholds, and sets the default health check path to "/".
func NewLoadBalancedWebSvc(input *LoadBalancedWebSvcProps) *LoadBalancedWebSvc {
	defaultLbManifest := newDefaultLoadBalancedWebSvc()
	defaultLbManifest.Svc = Svc{
		Name: input.SvcName,
		Type: LoadBalancedWebService,
	}
	defaultLbManifest.Image = SvcImageWithPort{
		SvcImage: SvcImage{
			Build: input.Dockerfile,
		},
		Port: input.Port,
	}
	defaultLbManifest.LoadBalancedWebSvcConfig.RoutingRule.Path = input.Path
	defaultLbManifest.parser = template.New()
	return defaultLbManifest
}

// newDefaultLoadBalancedWebSvc returns an empty LoadBalancedWebSvc with only the default values set.
func newDefaultLoadBalancedWebSvc() *LoadBalancedWebSvc {
	return &LoadBalancedWebSvc{
		Svc:   Svc{},
		Image: SvcImageWithPort{},
		LoadBalancedWebSvcConfig: LoadBalancedWebSvcConfig{
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
func (a *LoadBalancedWebSvc) MarshalBinary() ([]byte, error) {
	content, err := a.parser.Parse(lbWebSvcManifestPath, *a)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// DockerfilePath returns the image build path.
func (a *LoadBalancedWebSvc) DockerfilePath() string {
	return a.Image.Build
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (a *LoadBalancedWebSvc) ApplyEnv(envName string) *LoadBalancedWebSvc {
	target, ok := a.Environments[envName]
	if !ok {
		return a
	}

	return &LoadBalancedWebSvc{
		Svc:   a.Svc,
		Image: a.Image,
		LoadBalancedWebSvcConfig: LoadBalancedWebSvcConfig{
			RoutingRule: a.RoutingRule.copyAndApply(target.RoutingRule),
			TaskConfig:  a.TaskConfig.copyAndApply(target.TaskConfig),
			LogsConfig: LogsConfig{
				LogRetention: a.LogRetention,
			},
		},
	}
}
