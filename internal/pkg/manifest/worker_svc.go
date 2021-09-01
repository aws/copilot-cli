// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	workerSvcManifestPath = "workloads/services/worker/manifest.yml"
)

// WorkerService holds the configuration to create a worker service.
type WorkerService struct {
	Workload            `yaml:",inline"`
	WorkerServiceConfig `yaml:",inline"`
	// Use *WorkerServiceConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*WorkerServiceConfig `yaml:",flow"`

	parser template.Parser
}

// WorkerServiceConfig holds the configuration that can be overridden per environments.
type WorkerServiceConfig struct {
	ImageConfig      ImageWithHealthcheck `yaml:"image,flow"`
	ImageOverride    `yaml:",inline"`
	TaskConfig       `yaml:",inline"`
	Logging          `yaml:"logging,flow"`
	Sidecars         map[string]*SidecarConfig `yaml:"sidecars"`
	Subscribe        SubscribeConfig           `yaml:"subscribe"`
	Network          NetworkConfig             `yaml:"network"`
	TaskDefOverrides []OverrideRule            `yaml:"taskdef_overrides"`
}

// SubscribeConfig represents the configurable options for setting up subscriptions.
type SubscribeConfig struct {
	Topics []TopicSubscription `yaml:"topics"`
	Queue  *SQSQueue           `yaml:"queue"`
}

// TopicSubscription represents the configurable options for setting up a SNS Topic Subscription.
type TopicSubscription struct {
	Name    string    `yaml:"name"`
	Service string    `yaml:"service"`
	Queue   *SQSQueue `yaml:"queue"`
}

// SQSQueue represents the configurable options for setting up a SQS Queue.
type SQSQueue struct {
	Retention  *time.Duration   `yaml:"retention"`
	Delay      *time.Duration   `yaml:"delay"`
	Timeout    *time.Duration   `yaml:"timeout"`
	DeadLetter *DeadLetterQueue `yaml:"dead_letter"`
}

// DeadLetterQueue represents the configurable options for setting up a Dead-Letter Queue.
type DeadLetterQueue struct {
	Tries *uint16 `yaml:"tries"`
}

// WorkerServiceProps represents the configuration needed to create a worker service.
type WorkerServiceProps struct {
	WorkloadProps
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
	Topics      []TopicSubscription   // Optional topics for subscriptions
}

// NewWorkerService applies the props to a default Worker service configuration with
// minimal cpu/memory thresholds, single replica, no healthcheck, and then returns it.
func NewWorkerService(props WorkerServiceProps) *WorkerService {
	svc := newDefaultWorkerService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.WorkerServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.WorkerServiceConfig.ImageConfig.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.WorkerServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	svc.WorkerServiceConfig.Subscribe.Topics = props.Topics
	svc.parser = template.New()
	return svc
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *WorkerService) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(workerSvcManifestPath, *s, template.WithFuncs(map[string]interface{}{
		"fmtSlice":   template.FmtSliceFunc,
		"quoteSlice": template.QuoteSliceFunc,
	}))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (s *WorkerService) BuildRequired() (bool, error) {
	return requiresBuild(s.ImageConfig.Image)
}

// BuildArgs returns a docker.BuildArguments object for the service given a workspace root directory
func (s *WorkerService) BuildArgs(wsRoot string) *DockerBuildArgs {
	return s.ImageConfig.BuildConfig(wsRoot)
}

// Subscriptions returns a list of TopicSubscriotion objects which represent the SNS topics the service
// receives messages from.
func (s *WorkerService) Subscriptions() []TopicSubscription {
	return s.Subscribe.Topics
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s WorkerService) ApplyEnv(envName string) (WorkloadManifest, error) {
	overrideConfig, ok := s.Environments[envName]
	if !ok {
		return &s, nil
	}

	if overrideConfig == nil {
		return &s, nil
	}

	// Apply overrides to the original service s.
	for _, t := range defaultTransformers {
		err := mergo.Merge(&s, WorkerService{
			WorkerServiceConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))

		if err != nil {
			return nil, err
		}
	}
	s.Environments = nil
	return &s, nil
}

// newDefaultWorkerService returns a Worker service with minimal task sizes and a single replica.
func newDefaultWorkerService() *WorkerService {
	return &WorkerService{
		Workload: Workload{
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{},
			Subscribe:   SubscribeConfig{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: aws.String(PublicSubnetPlacement),
				},
			},
		},
	}
}
