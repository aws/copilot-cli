// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type scheduledJobParser interface {
	ParseScheduledJob(template.WorkloadOpts) (*template.Content, error)
}

// ScheduledJob represents the configuration needed to create a Cloudformation stack from a
// scheduled job manfiest.
type ScheduledJob struct {
	*wkld
	manifest *manifest.ScheduledJob

	parser scheduledJobParser
}

// NewScheduledJob creates a new ScheduledJob stack from a manifest file.
func NewScheduledJob(mft *manifest.ScheduledJob, env, app string, rc RuntimeConfig) (*ScheduledJob, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	envManifest, err := mft.ApplyEnv(env)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %w", env, err)
	}
	return &ScheduledJob{
		wkld: &wkld{
			name:   aws.StringValue(mft.Name),
			env:    env,
			app:    app,
			tc:     envManifest.ScheduledJobConfig.TaskConfig,
			rc:     rc,
			parser: parser,
			addons: addons,
		},
		manifest: envManifest,

		parser: parser,
	}, nil
}

// Template returns the CloudFormation template for the scheduled job.
func (j *ScheduledJob) Template() (string, error) {

	outputs, err := j.addonsOutputs()
	if err != nil {
		return "", err
	}

	sidecars, err := j.manifest.Sidecar.Options()
	if err != nil {
		return "", fmt.Errorf("convert the sidecar configuration for job %s: %w", j.name, err)
	}

	content, err := j.parser.ParseScheduledJob(template.WorkloadOpts{
		Variables:          j.manifest.Variables,
		Secrets:            j.manifest.Secrets,
		NestedStack:        outputs,
		Sidecars:           sidecars,
		ScheduleExpression: "",  // TODO: write the manifest.AWSSchedule() method
		StateMachine:       nil, // TODO: write the manifest.StateMachine() method
		LogConfig:          j.manifest.LogConfigOpts(),
	})
	if err != nil {
		return "", fmt.Errorf("parse scheduled job template: %w", err)
	}
	return content.String(), nil
}
