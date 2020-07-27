// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package task provides support for running Amazon ECS tasks.
package task

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"time"
)

// VpcGetter gets subnets and security groups.
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

// TaskRunner runs the tasks and wait for it to start.
type TaskRunner interface {
	RunTask(input ecs.RunTaskInput) ([]*ecs.Task, error)
}

type Task struct {
	TaskARN string
	ClusterARN string
	StartedAt time.Time
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

func tasks(ecsTasks []*ecs.Task) []*Task {
	tasks := make([]*Task, len(ecsTasks))
	for idx, task := range ecsTasks {
		tasks[idx] = &Task{
			TaskARN: aws.StringValue(task.TaskArn),
			ClusterARN: aws.StringValue(task.ClusterArn),
			StartedAt: aws.TimeValue(task.StartedAt),
		}
	}
	return tasks
}
