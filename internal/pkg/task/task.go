// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package task provides methods to run task in different configurations.
package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
)

type vpcGetter interface {
	GetSubnetIDs(filters ...ec2.Filter) ([]string, error)
	GetSecurityGroups(filters ...ec2.Filter) ([]string, error)
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

type defaultClusterGetter interface {
	DefaultCluster() (string, error)
}

type taskRunner interface {
	RunTask(input ecs.RunTaskInput) ([]string, error)
}

const (
	startedBy           = "copilot-task"
)

var (
	errNoSubnetFound     = errors.New("no subnets found")
	fmtTaskFamilyName    = "copilot-%s"
)

// Task gets information required to run a task.
type Task struct {
	count     int
	groupName string

	vpcGetter      vpcGetter
	resourceGetter resourceGetter
	clusterGetter  defaultClusterGetter
	starter        taskRunner
}

// NewTaskConfig contains fields required to initialize a task struct.
type NewTaskConfig struct {
	// Count is the count of the tasks to be run
	Count int
	// GroupName is the name of the task group
	GroupName string
}

// NewTask initializes a new task struct.
func NewTask(config NewTaskConfig) (*Task, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return &Task{
		count:     config.Count,
		groupName: config.GroupName,

		vpcGetter:      ec2.New(sess),
		resourceGetter: resourcegroups.New(sess),
		clusterGetter:  ecs.New(sess),
		starter:        ecs.New(sess),
	}, nil
}

// RunInDefaultVPC runs tasks in default cluster and default subnets, and returns the task ARNs.
func (t *Task) RunInDefaultVPC() ([]string, error) {
	// get the default cluster
	cluster, err := t.clusterGetter.DefaultCluster()
	if err != nil {
		return nil, fmt.Errorf("get default cluster: %w", err)
	}

	// get the default subnets
	subnets, err := t.vpcGetter.GetSubnetIDs(ec2.FilterForDefaultVPCSubnets)
	if err != nil {
		return nil, fmt.Errorf("get default subnet IDs: %w", err)
	}
	if len(subnets) == 0 {
		return nil, errNoSubnetFound
	}

	arns, err := t.starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          t.count,
		Subnets:        subnets,
		TaskFamilyName: taskFamilyName(t.groupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("run task %s: %w", t.groupName, err)
	}

	return arns, nil
}

func taskFamilyName(groupName string) string{
	return fmt.Sprintf(fmtTaskFamilyName, groupName)
}
