// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	// ScheduledJobType is a recurring ECS Fargate task which runs on a schedule.
	ScheduledJobType = "Scheduled Job"
)

const (
	scheduledJobManifestPath = "workloads/jobs/scheduled-job/manifest.yml"
)

// JobTypes holds the valid job "architectures"
var JobTypes = []string{
	ScheduledJobType,
}

// ScheduledJob holds the configuration to build a container image that is run
// periodically in a given environment with timeout and retry logic.
type ScheduledJob struct {
	Workload           `yaml:",inline"`
	ScheduledJobConfig `yaml:",inline"`
	Environments       map[string]*ScheduledJobConfig `yaml:",flow"`

	parser template.Parser
}

// ScheduledJobConfig holds the configuration for a scheduled job
type ScheduledJobConfig struct {
	ImageConfig             ImageWithHealthcheck `yaml:"image,flow"`
	ImageOverride           `yaml:",inline"`
	TaskConfig              `yaml:",inline"`
	*Logging                `yaml:"logging,flow"`
	Sidecars                map[string]*SidecarConfig `yaml:"sidecars"`
	On                      JobTriggerConfig          `yaml:"on,flow"`
	JobFailureHandlerConfig `yaml:",inline"`
	Network                 *NetworkConfig `yaml:"network"`
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
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
	Retries     int
}

// newDefaultScheduledJob returns an empty ScheduledJob with only the default values set.
func newDefaultScheduledJob() *ScheduledJob {
	return &ScheduledJob{
		Workload: Workload{
			Type: aws.String(ScheduledJobType),
		},
		ScheduledJobConfig: ScheduledJobConfig{
			ImageConfig: ImageWithHealthcheck{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
				},
			},
			Network: &NetworkConfig{
				VPC: &vpcConfig{
					Placement: stringP(PublicSubnetPlacement),
				},
			},
		},
	}
}

// NewScheduledJob creates a new scheduled job object.
func NewScheduledJob(props *ScheduledJobProps) *ScheduledJob {
	job := newDefaultScheduledJob()
	// Apply overrides.
	job.Name = stringP(props.Name)
	job.ImageConfig.Build.BuildArgs.Dockerfile = stringP(props.Dockerfile)
	job.ImageConfig.Location = stringP(props.Image)
	job.ImageConfig.HealthCheck = props.HealthCheck
	job.On.Schedule = stringP(props.Schedule)
	if props.Retries != 0 {
		job.Retries = aws.Int(props.Retries)
	}
	job.Timeout = stringP(props.Timeout)
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

// ApplyEnv returns the manifest with environment overrides.
func (j ScheduledJob) ApplyEnv(envName string) (WorkloadManifest, error) {
	overrideConfig, ok := j.Environments[envName]
	if !ok {
		return &j, nil
	}
	// Apply overrides to the original job
	err := mergo.Merge(&j, ScheduledJob{
		ScheduledJobConfig: *overrideConfig,
	}, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue, mergo.WithTransformers(workloadTransformer{}))

	if err != nil {
		return nil, err
	}
	j.Environments = nil
	return &j, nil
}

// BuildArgs returns a docker.BuildArguments object for the job given a workspace root.
func (j *ScheduledJob) BuildArgs(wsRoot string) *DockerBuildArgs {
	return j.ImageConfig.BuildConfig(wsRoot)
}

// BuildRequired returns if the service requires building from the local Dockerfile.
func (j *ScheduledJob) BuildRequired() (bool, error) {
	return requiresBuild(j.ImageConfig.Image)
}

// JobDockerfileBuildRequired returns if the job container image should be built from local Dockerfile.
func JobDockerfileBuildRequired(job interface{}) (bool, error) {
	return dockerfileBuildRequired("job", job)
}
