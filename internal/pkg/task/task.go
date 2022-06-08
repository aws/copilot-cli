// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package task provides support for running Amazon ECS tasks.
package task

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/describe"
)

// VPCGetter wraps methods of getting VPC info.
type VPCGetter interface {
	SubnetIDs(filters ...ec2.Filter) ([]string, error)
	SecurityGroups(filters ...ec2.Filter) ([]string, error)
}

// ClusterGetter wraps the method of getting a cluster ARN.
type ClusterGetter interface {
	ClusterARN(app, env string) (string, error)
}

// DefaultClusterGetter wraps the method of getting a default cluster ARN.
type DefaultClusterGetter interface {
	DefaultCluster() (string, error)
}

type EnvironmentDescriber interface {
	Describe() (*describe.EnvDescription, error)
}

// NonZeroExitCodeGetter wraps the method of getting a non-zero exit code of a task.
type NonZeroExitCodeGetter interface {
	HasNonZeroExitCode([]string, string) error
}

// Runner wraps the method of running tasks.
type Runner interface {
	RunTask(input ecs.RunTaskInput) ([]*ecs.Task, error)
}

// Task represents a one-off workload that runs until completed or an error occurs.
type Task struct {
	TaskARN    string
	ClusterARN string
	StartedAt  *time.Time
	ENI        string
}

const (
	startedBy = "copilot-task"

	// Platform options.
	osLinux             = template.OSLinux
	osWindowsServerFull = template.OSWindowsServerFull
	osWindowsServerCore = template.OSWindowsServerCore

	archX86   = template.ArchX86
	archARM64 = template.ArchARM64
)

var (
	ValidWindowsOSs   = []string{osWindowsServerCore, osWindowsServerFull}
	ValidCFNPlatforms = []string{dockerengine.PlatformString(osWindowsServerCore, archX86), dockerengine.PlatformString(osWindowsServerFull, archX86), dockerengine.PlatformString(osLinux, archX86), dockerengine.PlatformString(osLinux, archARM64)}

	fmtTaskFamilyName = "copilot-%s"
)

// IsValidWindowsOS determines if the OS value is an accepted CFN Windows value.
func IsValidWindowsOS(os string) bool {
	for _, validWindowsOS := range ValidWindowsOSs {
		if os == validWindowsOS {
			return true
		}
	}
	return false
}

func taskFamilyName(groupName string) string {
	return fmt.Sprintf(fmtTaskFamilyName, groupName)
}

func newTaskFromECS(ecsTask *ecs.Task) *Task {
	taskARN := aws.StringValue(ecsTask.TaskArn)
	eni, _ := ecsTask.ENI() //  Best-effort parse the ENI. If we can't find an IP address, we won't show it to the customers instead of erroring.
	return &Task{
		TaskARN:    taskARN,
		ClusterARN: aws.StringValue(ecsTask.ClusterArn),
		StartedAt:  ecsTask.StartedAt,
		ENI:        eni,
	}
}

func convertECSTasks(ecsTasks []*ecs.Task) []*Task {
	tasks := make([]*Task, len(ecsTasks))
	for idx, ecsTask := range ecsTasks {
		tasks[idx] = newTaskFromECS(ecsTask)
	}
	// Even if ENI information is not found for some tasks, we still want to return the other information as we can
	return tasks
}
