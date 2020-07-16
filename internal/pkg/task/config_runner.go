// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

// NetworkConfigRunner runs an Amazon ECS task in the specified network configuration and the default cluster.
type NetworkConfigRunner struct {
	// Count of the tasks to be launched.
	Count int
	// Group Name of the tasks that use the same task definition.
	GroupName string

	// Network configuration
	Subnets        []string
	SecurityGroups []string

	// Interfaces to interact with dependencies. Must not be nil.
	ClusterGetter DefaultClusterGetter
	Starter       TaskRunner
}

// Run runs tasks in the subnets and the security groups, and returns the task ARNs.
func (r *NetworkConfigRunner) Run() ([]string, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.ClusterGetter.DefaultCluster()
	if err != nil {
		return nil, fmt.Errorf("get default cluster: %w", err)
	}

	arns, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          r.Count,
		Subnets:        r.Subnets,
		SecurityGroups: r.SecurityGroups,
		TaskFamilyName: taskFamilyName(r.GroupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("run task %s: %w", r.GroupName, err)
	}

	return arns, nil
}

func (r *NetworkConfigRunner) validateDependencies() error {
	if r.ClusterGetter == nil {
		return errors.New("cluster getter is not set")
	}

	if r.Starter == nil {
		return errors.New("starter is not set")
	}

	return nil
}
