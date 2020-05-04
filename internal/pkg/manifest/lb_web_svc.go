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

// LoadBalancedWebService holds the configuration to build a container image with an exposed port that receives
// requests through a load balancer with AWS Fargate as the compute engine.
type LoadBalancedWebService struct {
	Service                      `yaml:",inline"`
	Image                        ServiceImageWithPort `yaml:",flow"`
	LoadBalancedWebServiceConfig `yaml:",inline"`
	Environments                 map[string]LoadBalancedWebServiceConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// LoadBalancedWebServiceConfig represents a load balanced web service with AWS Fargate as compute.
type LoadBalancedWebServiceConfig struct {
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

// LoadBalancedWebServiceProps contains properties for creating a new load balanced fargate service manifest.
type LoadBalancedWebServiceProps struct {
	*ServiceProps
	Path string
	Port uint16
}

// NewLoadBalancedWebService creates a new public load balanced web service, receives all the requests from the load balancer,
// has a single task with minimal CPU and memory thresholds, and sets the default health check path to "/".
func NewLoadBalancedWebService(input *LoadBalancedWebServiceProps) *LoadBalancedWebService {
	defaultLbManifest := newDefaultLoadBalancedWebService()
	defaultLbManifest.Service = Service{
		Name: input.Name,
		Type: LoadBalancedWebServiceType,
	}
	defaultLbManifest.Image = ServiceImageWithPort{
		ServiceImage: ServiceImage{
			Build: input.Dockerfile,
		},
		Port: input.Port,
	}
	defaultLbManifest.LoadBalancedWebServiceConfig.RoutingRule.Path = input.Path
	defaultLbManifest.parser = template.New()
	return defaultLbManifest
}

// newDefaultLoadBalancedWebService returns an empty LoadBalancedWebService with only the default values set.
func newDefaultLoadBalancedWebService() *LoadBalancedWebService {
	return &LoadBalancedWebService{
		Service: Service{},
		Image:   ServiceImageWithPort{},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
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
func (s *LoadBalancedWebService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(lbWebSvcManifestPath, *s)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// DockerfilePath returns the image build path.
func (s *LoadBalancedWebService) DockerfilePath() string {
	return s.Image.Build
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s *LoadBalancedWebService) ApplyEnv(envName string) *LoadBalancedWebService {
	target, ok := s.Environments[envName]
	if !ok {
		return s
	}

	return &LoadBalancedWebService{
		Service: s.Service,
		Image:   s.Image,
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			RoutingRule: s.RoutingRule.copyAndApply(target.RoutingRule),
			TaskConfig:  s.TaskConfig.copyAndApply(target.TaskConfig),
			LogsConfig: LogsConfig{
				LogRetention: s.LogRetention,
			},
		},
	}
}
