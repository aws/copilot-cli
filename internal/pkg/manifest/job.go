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
	ImageConfig    Image `yaml:"image,flow"`
	TaskConfig     `yaml:",inline"`
	*Logging       `yaml:"logging,flow"`
	Sidecar        `yaml:",inline"`
	ScheduleConfig `yaml:",inline"`
}

// ScheduleConfig holds the fields necessary to describe a scheduled job's execution frequency and error handling.
type ScheduleConfig struct {
	Schedule string `yaml:"schedule"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

// ScheduledJobProps contains properties for creating a new scheduled job manifest.
type ScheduledJobProps struct {
	*WorkloadProps
	Schedule string
	Timeout  string
	Retries  int
}

// LogConfigOpts converts the job's Firelens configuration into a format parsable by the templates pkg.
func (lc *ScheduledJobConfig) LogConfigOpts() *template.LogConfigOpts {
	if lc.Logging == nil {
		return nil
	}
	return lc.logConfigOpts()
}

// newDefaultScheduledJob returns an empty ScheduledJob with only the default values set.
func newDefaultScheduledJob() *ScheduledJob {
	return &ScheduledJob{
		Workload: Workload{
			Type: aws.String(ScheduledJobType),
		},
		ScheduledJobConfig: ScheduledJobConfig{
			ImageConfig: Image{},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(512),
				Count: Count{
					Value: aws.Int(1),
				},
			},
		},
	}
}

// NewScheduledJob creates a new
func NewScheduledJob(props *ScheduledJobProps) *ScheduledJob {
	job := newDefaultScheduledJob()
	// Apply overrides.
	job.Name = aws.String(props.Name)
	job.ScheduledJobConfig.ImageConfig.Build.BuildArgs.Dockerfile = aws.String(props.Dockerfile)
	job.Schedule = props.Schedule
	job.Retries = props.Retries
	job.Timeout = props.Timeout

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
func (j ScheduledJob) ApplyEnv(envName string) (*ScheduledJob, error) {
	overrideConfig, ok := j.Environments[envName]
	if !ok {
		return &j, nil
	}
	// Apply overrides to the original job
	err := mergo.Merge(&j, ScheduledJob{
		ScheduledJobConfig: *overrideConfig,
	}, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
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
	return requiresBuild(j.ImageConfig)
}

// JobDockerfileBuildRequired returns if the job container image should be built from local Dockerfile.
func JobDockerfileBuildRequired(job interface{}) (bool, error) {
	return dockerfileBuildRequired("job", job)
}
