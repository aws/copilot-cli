// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"
)

const (
	workerSvcManifestPath = "workloads/services/worker/manifest.yml"
)

var (
	errUnmarshalQueueOpts  = errors.New(`cannot unmarshal "queue" field into bool or map`)
	errUnmarshalFifoConfig = errors.New(`unable to unmarshal "fifo" field into boolean or compose-style map`)
)

// WorkerService holds the configuration to create a worker service.
type WorkerService struct {
	Workload            `yaml:",inline"`
	WorkerServiceConfig `yaml:",inline"`
	// Use *WorkerServiceConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*WorkerServiceConfig `yaml:",flow"`
	parser       template.Parser
}

// Publish returns the list of topics where notifications can be published.
func (s *WorkerService) Publish() []Topic {
	return s.WorkerServiceConfig.PublishConfig.publishedTopics()
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
	DeployConfig     WorkerDeploymentConfig    `yaml:"deployment"`
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
	Retention  *time.Duration          `yaml:"retention"`
	Delay      *time.Duration          `yaml:"delay"`
	Timeout    *time.Duration          `yaml:"timeout"`
	DeadLetter DeadLetterQueue         `yaml:"dead_letter"`
	FIFO       FIFOAdvanceConfigOrBool `yaml:"fifo"`
}

// FIFOAdvanceConfigOrBool represents the configurable options for fifo queues.
type FIFOAdvanceConfigOrBool struct {
	Enable   *bool
	Advanced FIFOAdvanceConfig
}

// IsEmpty returns true if the FifoAdvanceConfigOrBool struct has all nil values.
func (f *FIFOAdvanceConfigOrBool) IsEmpty() bool {
	return f.Enable == nil && f.Advanced.IsEmpty()
}

// IsEnabled returns true if the FifoAdvanceConfigOrBool struct has all nil values.
func (f *FIFOAdvanceConfigOrBool) IsEnabled() bool {
	return aws.BoolValue(f.Enable) || !f.Advanced.IsEmpty()
}

// FIFOAdvanceConfig represents the advanced fifo queue config.
type FIFOAdvanceConfig struct {
	ContentBasedDeduplication *bool   `yaml:"content_based_deduplication"`
	DeduplicationScope        *string `yaml:"deduplication_scope"`
	FIFOThroughputLimit       *string `yaml:"throughput_limit"`
	HighThroughputFifo        *bool   `yaml:"high_throughput"`
}

// IsEmpty returns true if the FifoAdvanceConfig struct has all nil values.
func (f *FIFOAdvanceConfig) IsEmpty() bool {
	return f.FIFOThroughputLimit == nil && f.HighThroughputFifo == nil &&
		f.DeduplicationScope == nil && f.ContentBasedDeduplication == nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the FifoAdvanceConfigOrBool
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (t *FIFOAdvanceConfigOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&t.Advanced); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}
	if !t.Advanced.IsEmpty() {
		return nil
	}
	if err := value.Decode(&t.Enable); err != nil {
		return errUnmarshalFifoConfig
	}
	return nil
}

// IsEmpty returns empty if the struct has all zero members.
func (q *SQSQueue) IsEmpty() bool {
	return q.Retention == nil && q.Delay == nil && q.Timeout == nil &&
		q.DeadLetter.IsEmpty() && q.FIFO.IsEmpty()
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
	Queue       SQSQueue             // Optional queue configuration.
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
	if len(props.Topics) > 0 {
		setSubscriptionQueueDefaults(props.Topics, &svc.WorkerServiceConfig.Subscribe.Queue)
	}
	svc.WorkerServiceConfig.Subscribe.Topics = props.Topics
	svc.WorkerServiceConfig.Platform = props.Platform
	for _, envName := range props.PrivateOnlyEnvironments {
		svc.Environments[envName] = &WorkerServiceConfig{
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PrivateSubnetPlacement),
					},
				},
			},
		}
	}
	svc.parser = template.New()
	return svc
}

// setSubscriptionQueueDefaults function modifies the manifest to have
//  1. FIFO Topic names without ".fifo" suffix.
//  2. If there are both FIFO and Standard topic subscriptions are specified then set
//     default events queue to FIFO and add standard topic-specific queue for all the standard topic subscriptions.
//  3. If there are only Standard topic subscriptions are specified then do nothing and return.
func setSubscriptionQueueDefaults(topics []TopicSubscription, eventsQueue *SQSQueue) {
	var isFIFOEnabled bool
	for _, topic := range topics {
		if isFIFO(aws.StringValue(topic.Name)) {
			isFIFOEnabled = true
			break
		}
	}
	if !isFIFOEnabled {
		return
	}
	eventsQueue.FIFO.Enable = aws.Bool(true)
	for idx, topic := range topics {
		if isFIFO(aws.StringValue(topic.Name)) {
			topics[idx].Name = aws.String(strings.TrimSuffix(aws.StringValue(topic.Name), ".fifo"))
		} else {
			topics[idx].Queue.Enabled = aws.Bool(true)
		}
	}
}

func isFIFO(topic string) bool {
	return strings.HasSuffix(topic, ".fifo")
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

// BuildArgs returns a docker.BuildArguments object for the service given a context directory
func (s *WorkerService) BuildArgs(contextDir string) (map[string]*DockerBuildArgs, error) {
	required, err := requiresBuild(s.ImageConfig.Image)
	if err != nil {
		return nil, err
	}
	// Creating an map to store buildArgs of all sidecar images and main container image.
	buildArgsPerContainer := make(map[string]*DockerBuildArgs, len(s.Sidecars)+1)
	if required {
		buildArgsPerContainer[aws.StringValue(s.Name)] = s.ImageConfig.Image.BuildConfig(contextDir)
	}
	return buildArgs(contextDir, buildArgsPerContainer, s.Sidecars)
}

// EnvFiles returns the locations of all env files against the ws root directory.
// This method returns a map[string]string where the keys are container names
// and the values are either env file paths or empty strings.
func (s *WorkerService) EnvFiles() map[string]string {
	return envFiles(s.Name, s.TaskConfig, s.Logging, s.Sidecars)
}

// ContainerDependencies returns a map of ContainerDependency objects for the WorkerService
// including dependencies for its main container, any logging sidecar, and additional sidecars.
func (s *WorkerService) ContainerDependencies() map[string]ContainerDependency {
	return containerDependencies(aws.StringValue(s.Name), s.ImageConfig.Image, s.Logging, s.Sidecars)
}

// Subscriptions returns a list of TopicSubscriotion objects which represent the SNS topics the service
// receives messages from. This method also appends ".fifo" to the topics and returns a new set of subs.
func (s *WorkerService) Subscriptions() []TopicSubscription {
	var subs []TopicSubscription
	for _, topic := range s.Subscribe.Topics {
		topicSubscription := topic
		// if condition appends .fifo suffix to the topic which doesn't have topic specific queue and subscribing to default FIFO queue.
		if topic.Queue.IsEmpty() && !s.Subscribe.Queue.IsEmpty() && s.Subscribe.Queue.FIFO.IsEnabled() {
			topicSubscription.Name = aws.String(aws.StringValue(topic.Name) + ".fifo")
		} else if !topic.Queue.IsEmpty() && !topic.Queue.Advanced.IsEmpty() && topic.Queue.Advanced.FIFO.IsEnabled() { // else if condition appends .fifo suffix to the topic which has topic specific FIFO queue configuration.
			topicSubscription.Name = aws.String(aws.StringValue(topic.Name) + ".fifo")
		}
		subs = append(subs, topicSubscription)
	}
	return subs
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
			Type: aws.String(manifestinfo.WorkerServiceType),
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
						workloadType: manifestinfo.WorkerServiceType,
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
		Environments: map[string]*WorkerServiceConfig{},
	}
}

// ExposedPorts returns all the ports that are sidecar container ports available to receive traffic.
func (ws *WorkerService) ExposedPorts() (ExposedPortsIndex, error) {
	exposedPorts := make(map[uint16]ExposedPort)
	for name, sidecar := range ws.Sidecars {
		newExposedPorts, err := sidecar.exposePorts(exposedPorts, name)
		if err != nil {
			return ExposedPortsIndex{}, err
		}
		maps.Copy(exposedPorts, newExposedPorts)
	}
	portsForContainer, containerForPort := prepareParsedExposedPortsMap(exposedPorts)
	return ExposedPortsIndex{
		PortsForContainer: portsForContainer,
		ContainerForPort:  containerForPort,
	}, nil
}
