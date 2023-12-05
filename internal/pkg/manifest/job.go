// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"maps"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	scheduledJobManifestPath = "workloads/jobs/scheduled-job/manifest.yml"
)

// ScheduledJob holds the configuration to build a container image that is run
// periodically in a given environment with timeout and retry logic.
type ScheduledJob struct {
	Workload           `yaml:",inline"`
	ScheduledJobConfig `yaml:",inline"`
	Environments       map[string]*ScheduledJobConfig `yaml:",flow"`
	parser             template.Parser
}

func (s *ScheduledJob) subnets() *SubnetListOrArgs {
	return &s.Network.VPC.Placement.Subnets
}

// ScheduledJobConfig holds the configuration for a scheduled job
type ScheduledJobConfig struct {
	ImageConfig             ImageWithHealthcheck `yaml:"image,flow"`
	ImageOverride           `yaml:",inline"`
	TaskConfig              `yaml:",inline"`
	Logging                 Logging                   `yaml:"logging,flow"`
	Sidecars                map[string]*SidecarConfig `yaml:"sidecars"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
	On                      JobTriggerConfig          `yaml:"on,flow"`
	JobFailureHandlerConfig `yaml:",inline"`
	Network                 NetworkConfig  `yaml:"network"`
	PublishConfig           PublishConfig  `yaml:"publish"`
	TaskDefOverrides        []OverrideRule `yaml:"taskdef_overrides"`
}

// JobTriggerConfig represents the configuration for the event that triggers the job.
type JobTriggerConfig struct {
	Schedule *string `yaml:"schedule"`
}

// JobFailureHandlerConfig represents the error handling configuration for the job.
type JobFailureHandlerConfig struct {
	Timeout *string `yaml:"timeout"`
	Retries *int    `yaml:"retries"`
}

// ScheduledJobProps contains properties for creating a new scheduled job manifest.
type ScheduledJobProps struct {
	*WorkloadProps
	Schedule    string
	Timeout     string
	HealthCheck ContainerHealthCheck // Optional healthcheck configuration.
	Platform    PlatformArgsOrString // Optional platform configuration.
	Retries     int
}

// NewScheduledJob creates a new scheduled job object.
func NewScheduledJob(props *ScheduledJobProps) *ScheduledJob {
	job := newDefaultScheduledJob()
	// Apply overrides.
	job.Name = stringP(props.Name)
	job.ImageConfig.Image.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	job.ImageConfig.Image.Location = stringP(props.Image)
	job.ImageConfig.HealthCheck = props.HealthCheck
	job.Platform = props.Platform
	if isWindowsPlatform(props.Platform) {
		job.TaskConfig.CPU = aws.Int(MinWindowsTaskCPU)
		job.TaskConfig.Memory = aws.Int(MinWindowsTaskMemory)
	}
	job.On.Schedule = stringP(props.Schedule)
	if props.Retries != 0 {
		job.Retries = aws.Int(props.Retries)
	}
	job.Timeout = stringP(props.Timeout)
	for _, envName := range props.PrivateOnlyEnvironments {
		job.Environments[envName] = &ScheduledJobConfig{
			Network: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PrivateSubnetPlacement),
					},
				},
			},
		}
	}
	job.parser = template.New()
	return job
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (j *ScheduledJob) MarshalBinary() ([]byte, error) {
	content, err := j.parser.Parse(scheduledJobManifestPath, *j)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func (j ScheduledJob) applyEnv(envName string) (workloadManifest, error) {
	overrideConfig, ok := j.Environments[envName]
	if !ok {
		return &j, nil
	}

	// Apply overrides to the original job
	for _, t := range defaultTransformers {
		err := mergo.Merge(&j, ScheduledJob{
			ScheduledJobConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))
		if err != nil {
			return nil, err
		}
	}
	j.Environments = nil
	return &j, nil
}

func (s *ScheduledJob) requiredEnvironmentFeatures() []string {
	var features []string
	features = append(features, s.Network.requiredEnvFeatures()...)
	features = append(features, s.Storage.requiredEnvFeatures()...)
	return features
}

// Publish returns the list of topics where notifications can be published.
func (j *ScheduledJob) Publish() []Topic {
	return j.ScheduledJobConfig.PublishConfig.publishedTopics()
}

// BuildArgs returns a docker.BuildArguments object for the job given a context directory.
func (j *ScheduledJob) BuildArgs(contextDir string) (map[string]*DockerBuildArgs, error) {
	required, err := requiresBuild(j.ImageConfig.Image)
	if err != nil {
		return nil, err
	}
	// Creating an map to store buildArgs of all sidecar images and main container image.
	buildArgsPerContainer := make(map[string]*DockerBuildArgs, len(j.Sidecars)+1)
	if required {
		buildArgsPerContainer[aws.StringValue(j.Name)] = j.ImageConfig.Image.BuildConfig(contextDir)
	}
	return buildArgs(contextDir, buildArgsPerContainer, j.Sidecars)
}

// EnvFiles returns the locations of all env files against the ws root directory.
// This method returns a map[string]string where the keys are container names
// and the values are either env file paths or empty strings.
func (j *ScheduledJob) EnvFiles() map[string]string {
	return envFiles(j.Name, j.TaskConfig, j.Logging, j.Sidecars)
}

// ContainerDependencies returns a map of ContainerDependency objects for ScheduledJob
// including dependencies for its main container, any logging sidecar, and additional sidecars.
func (s *ScheduledJob) ContainerDependencies() map[string]ContainerDependency {
	return containerDependencies(aws.StringValue(s.Name), s.ImageConfig.Image, s.Logging, s.Sidecars)
}

// newDefaultScheduledJob returns an empty ScheduledJob with only the default values set.
func newDefaultScheduledJob() *ScheduledJob {
	return &ScheduledJob{
		Workload: Workload{
			Type: aws.String(manifestinfo.ScheduledJobType),
		},
		ScheduledJobConfig: ScheduledJobConfig{
			ImageConfig: ImageWithHealthcheck{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
					AdvancedCount: AdvancedCount{ // Leave advanced count empty while passing down the type of the workload.
						workloadType: manifestinfo.ScheduledJobType,
					},
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
		Environments: map[string]*ScheduledJobConfig{},
	}
}

// ExposedPorts returns all the ports that are sidecar container ports available to receive traffic.
func (j *ScheduledJob) ExposedPorts() (ExposedPortsIndex, error) {
	exposedPorts := make(map[uint16]ExposedPort)
	for name, sidecar := range j.Sidecars {
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
