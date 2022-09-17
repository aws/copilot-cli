// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"
)

const (
	workerSvcManifestPath = "workloads/services/worker/manifest.yml"
)

var (
	errUnmarshalQueueOpts = errors.New(`cannot unmarshal "queue" field into bool or map`)
)

// WorkerService holds the configuration to create a worker service.
type WorkerService struct {
	Workload            `yaml:",inline"`
	WorkerServiceConfig `yaml:",inline"`
	// Use *WorkerServiceConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*WorkerServiceConfig `yaml:",flow"`

	parser template.Parser
}

// Publish returns the list of topics where notifications can be published.
func (s *WorkerService) Publish() []Topic {
	return s.WorkerServiceConfig.PublishConfig.Topics
}

func (s *WorkerService) subnets() *SubnetListOrArgs {
	return &s.Network.VPC.Placement.Subnets
}

// WorkerServiceConfig holds the configuration that can be overridden per environments.
type WorkerServiceConfig struct {
	ImageConfig      ImageWithHealthcheck `yaml:"image,flow"`
	ImageOverride    `yaml:",inline"`
	TaskConfig       `yaml:",inline"`
	Logging          Logging                   `yaml:"logging,flow"`
	Sidecars         map[string]*SidecarConfig `yaml:"sidecars"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
	Subscribe        SubscribeConfig           `yaml:"subscribe"`
	PublishConfig    PublishConfig             `yaml:"publish"`
	Network          NetworkConfig             `yaml:"network"`
	TaskDefOverrides []OverrideRule            `yaml:"taskdef_overrides"`
	DeployConfig     DeploymentConfiguration   `yaml:"deployment"`
	Observability    Observability             `yaml:"observability"`
}

// SubscribeConfig represents the configurable options for setting up subscriptions.
type SubscribeConfig struct {
	Topics []TopicSubscription `yaml:"topics"`
	Queue  SQSQueue            `yaml:"queue"`
}

// IsEmpty returns empty if the struct has all zero members.
func (s *SubscribeConfig) IsEmpty() bool {
	return s.Topics == nil && s.Queue.IsEmpty()
}

// TopicSubscription represents the configurable options for setting up a SNS Topic Subscription.
type TopicSubscription struct {
	Name         *string                `yaml:"name"`
	Service      *string                `yaml:"service"`
	FilterPolicy map[string]interface{} `yaml:"filter_policy"`
	Queue        SQSQueueOrBool         `yaml:"queue"`
}

// SQSQueueOrBool is a custom type which supports unmarshaling yaml which
// can either be of type bool or type SQSQueue.
type SQSQueueOrBool struct {
	Advanced SQSQueue
	Enabled  *bool
}

// IsEmpty returns empty if the struct has all zero members.
func (q *SQSQueueOrBool) IsEmpty() bool {
	return q.Advanced.IsEmpty() && q.Enabled == nil
}

// UnmarshalYAML implements the yaml(v3) interface. It allows SQSQueueOrBool to be specified as a
// string or a struct alternately.
func (q *SQSQueueOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&q.Advanced); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}
	if !q.Advanced.IsEmpty() {
		// Unmarshaled successfully to q.Advanced, unset q.Enabled, and return.
		q.Enabled = nil
		return nil
	}
	if err := value.Decode(&q.Enabled); err != nil {
		return errUnmarshalQueueOpts
	}
	return nil
}

// SQSQueue represents the configurable options for setting up a SQS Queue.
type SQSQueue struct {
	Retention                 *time.Duration  `yaml:"retention"`
	Delay                     *time.Duration  `yaml:"delay"`
	Timeout                   *time.Duration  `yaml:"timeout"`
	DeadLetter                DeadLetterQueue `yaml:"dead_letter"`
	ContentBasedDeduplication *bool           `yaml:"content_based_deduplication"`
	DeduplicationScope        *string         `yaml:"deduplication_scope"`
	FifoThroughputLimit       *string         `yaml:"fifo_throughput_limit"`
	HighThroughputFifo        *bool           `yaml:"high_throughput_fifo"`
	Type                      *string         `yaml:"type"`
}

// IsEmpty returns empty if the struct has all zero members.
func (q *SQSQueue) IsEmpty() bool {
	return q.Retention == nil && q.Delay == nil && q.Timeout == nil &&
		q.DeadLetter.IsEmpty() && q.FifoThroughputLimit == nil && q.HighThroughputFifo == nil &&
		q.DeduplicationScope == nil && q.ContentBasedDeduplication == nil && q.Type == nil
}

// DeadLetterQueue represents the configurable options for setting up a Dead-Letter Queue.
type DeadLetterQueue struct {
	Tries *uint16 `yaml:"tries"`
}

// IsEmpty returns empty if the struct has all zero members.
func (q *DeadLetterQueue) IsEmpty() bool {
	return q.Tries == nil
}

// WorkerServiceProps represents the configuration needed to create a worker service.
type WorkerServiceProps struct {
	WorkloadProps

	HealthCheck ContainerHealthCheck // Optional healthcheck configuration.
	Platform    PlatformArgsOrString // Optional platform configuration.
	Topics      []TopicSubscription  // Optional topics for subscriptions
}

// NewWorkerService applies the props to a default Worker service configuration with
// minimal cpu/memory thresholds, single replica, no healthcheck, and then returns it.
func NewWorkerService(props WorkerServiceProps) *WorkerService {
	svc := newDefaultWorkerService()
	// Apply overrides.
	svc.Name = stringP(props.Name)
	svc.WorkerServiceConfig.ImageConfig.Image.Location = stringP(props.Image)
	svc.WorkerServiceConfig.ImageConfig.Image.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	svc.WorkerServiceConfig.ImageConfig.HealthCheck = props.HealthCheck
	svc.WorkerServiceConfig.Platform = props.Platform
	if isWindowsPlatform(props.Platform) {
		svc.WorkerServiceConfig.TaskConfig.CPU = aws.Int(MinWindowsTaskCPU)
		svc.WorkerServiceConfig.TaskConfig.Memory = aws.Int(MinWindowsTaskMemory)
	}
	svc.WorkerServiceConfig.Subscribe.Topics = props.Topics
	svc.WorkerServiceConfig.Platform = props.Platform
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
	return s.ImageConfig.Image.BuildConfig(wsRoot)
}

// EnvFile returns the location of the env file against the ws root directory.
func (s *WorkerService) EnvFile() string {
	return aws.StringValue(s.TaskConfig.EnvFile)
}

// Subscriptions returns a list of TopicSubscriotion objects which represent the SNS topics the service
// receives messages from.
func (s *WorkerService) Subscriptions() []TopicSubscription {
	return s.Subscribe.Topics
}

// Rename is function
func (s *WorkerService) AttachFIFOSuffixToFIFOQueues() {
	for idx, topic := range s.Subscribe.Topics {
		// if condition appends .fifo suffix to the topic which doesn't have topic specific queue and subscribing to default FIFO queue.
		if topic.Queue.IsEmpty() && !s.Subscribe.Queue.IsEmpty() && s.Subscribe.Queue.Type != nil && strings.Compare(aws.StringValue(s.Subscribe.Queue.Type), "fifo") == 0 {
			s.Subscribe.Topics[idx].Name = aws.String(aws.StringValue(topic.Name) + ".fifo")
		} else if !topic.Queue.IsEmpty() && !topic.Queue.Advanced.IsEmpty() && topic.Queue.Advanced.Type != nil && strings.Compare(aws.StringValue(topic.Queue.Advanced.Type), "fifo") == 0 { // else if condition appends .fifo suffix to the topic which has topic specific FIFO queue configuration.
			s.Subscribe.Topics[idx].Name = aws.String(aws.StringValue(topic.Name) + ".fifo")
		}
		// if condition sets topic specific type to defaultQueueType (i.e. standard) in case customer doesn't define it in their topic specific queue.
		if !topic.Queue.IsEmpty() && !topic.Queue.Advanced.IsEmpty() && topic.Queue.Advanced.Type == nil {
			s.Subscribe.Topics[idx].Queue.Advanced.Type = aws.String(defaultQueueType)
		}
	}
	if s.Subscribe.Queue.IsEmpty() && s.Subscribe.StandardDefaultQueue() {
		s.Subscribe.Queue = SQSQueue{
			Type: aws.String(defaultQueueType),
		}
		//subscriptions.Queue = &template.SQSQueue{Type: aws.String(defaultQueueType)}
	} else if !s.Subscribe.Queue.IsEmpty() && s.Subscribe.Queue.Type == nil {
		s.Subscribe.Queue.Type = aws.String(defaultQueueType)
	}
}

// StandardDefaultQueue returns true if at least one standard topic exist and does not have its own queue configs.
func (s *SubscribeConfig) StandardDefaultQueue() bool {
	for _, t := range s.Topics {
		if t.Queue.IsEmpty() {
			return true
		}
	}
	return false
}

func (s WorkerService) applyEnv(envName string) (workloadManifest, error) {
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

func (s *WorkerService) requiredEnvironmentFeatures() []string {
	var features []string
	features = append(features, s.Network.requiredEnvFeatures()...)
	features = append(features, s.Storage.requiredEnvFeatures()...)
	return features
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
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: WorkerServiceType,
					},
				},
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			},
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PublicSubnetPlacement),
					},
				},
			},
		},
	}
}
