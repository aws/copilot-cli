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
	"github.com/aws/copilot-cli/internal/pkg/describe"
)

// VPCGetter wraps methods of getting VPC info.
type VPCGetter interface {
	SubnetIDs(filters ...ec2.Filter) ([]string, error)
	SecurityGroups(filters ...ec2.Filter) ([]string, error)
	PublicSubnetIDs(filters ...ec2.Filter) ([]string, error)
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
)

var (
	fmtTaskFamilyName = "copilot-%s"
)

func taskFamilyName(groupName string) string {
	return fmt.Sprintf(fmtTaskFamilyName, groupName)
}

func newTaskFromECS(ecsTask *ecs.Task) (*Task, error) {
	taskARN := aws.StringValue(ecsTask.TaskArn)
	eni, err := ecsTask.ENI()
	return &Task{
		TaskARN:    taskARN,
		ClusterARN: aws.StringValue(ecsTask.ClusterArn),
		StartedAt:  ecsTask.StartedAt,
		ENI:        eni,
	}, err
}

func concatenateENINotFoundErrors(errs []*ecs.ErrTaskENIInfoNotFound) error {
	e := &ErrENIInfoNotFoundForTasks{
		Errors: make([]*ecs.ErrTaskENIInfoNotFound, 0),
	}
	for _, err := range errs {
		if err != nil {
			e.Errors = append(e.Errors, err)
		}
	}
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}

func convertECSTasks(ecsTasks []*ecs.Task) ([]*Task, error) {
	eniNotFoundErrs := make([]*ecs.ErrTaskENIInfoNotFound, len(ecsTasks))
	tasks := make([]*Task, len(ecsTasks))
	for idx, ecsTask := range ecsTasks {
		task, err := newTaskFromECS(ecsTask)
		serr, ok := err.(*ecs.ErrTaskENIInfoNotFound)
		if err != nil && !ok {
			return nil, err
		}
		tasks[idx], eniNotFoundErrs[idx] = task, serr
	}
	// Even if ENI information is not found for some tasks, we still want to return the other information as we can
	return tasks, concatenateENINotFoundErrors(eniNotFoundErrs)
}
