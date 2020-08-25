// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package task provides support for running Amazon ECS tasks.
package task

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
)

// VPCGetter gets subnets and security groups.
type VPCGetter interface {
	SubnetIDs(filters ...ec2.Filter) ([]string, error)
	SecurityGroups(filters ...ec2.Filter) ([]string, error)
	PublicSubnetIDs(filters ...ec2.Filter) ([]string, error)
}

// ResourceGetter gets resources by tags.
type ResourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

// DefaultClusterGetter gets the default cluster.
type DefaultClusterGetter interface {
	DefaultCluster() (string, error)
}

// Runner runs the tasks and wait for it to start.
type Runner interface {
	RunTask(input ecs.RunTaskInput) ([]*ecs.Task, error)
}

// Task represents a one-off workload that runs until completed or an error occurs.
type Task struct {
	TaskARN    string
	ClusterARN string
	StartedAt  *time.Time
}

const (
	startedBy = "copilot-task"
)

var (
	fmtTaskFamilyName = "copilot-%s"
)

func taskFamilyName(groupName string) string {
	return fmt.Sprintf(fmtTaskFamilyName, groupName)
}

func newTaskFromECS(ecsTask *ecs.Task) *Task {
	return &Task{
		TaskARN:    aws.StringValue(ecsTask.TaskArn),
		ClusterARN: aws.StringValue(ecsTask.ClusterArn),
		StartedAt:  ecsTask.StartedAt,
	}
}

func convertECSTasks(ecsTasks []*ecs.Task) []*Task {
	tasks := make([]*Task, len(ecsTasks))
	for idx, ecsTask := range ecsTasks {
		tasks[idx] = newTaskFromECS(ecsTask)
	}
	return tasks
}
