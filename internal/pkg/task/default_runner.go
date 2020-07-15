// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

// DefaultVPCRunner can run an Amazon ECS task in the default VPC and the default cluster.
type DefaultVPCRunner struct {
	// Count of the tasks to be launched.
	Count     int
	// Group Name fo the tasks that use the same task definition.
	GroupName string

	// Interfaces to interact with dependencies. Must not be nil.
	VPCGetter     VPCGetter
	ClusterGetter DefaultClusterGetter
	Starter       TaskRunner
}

// Run runs tasks in default cluster and default subnets, and returns the task ARNs.
func (r *DefaultVPCRunner) Run() ([]string, error) {
	if err:= r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.ClusterGetter.DefaultCluster()
	if err != nil {
		return nil, fmt.Errorf("get default cluster: %w", err)
	}

	subnets, err := r.VPCGetter.GetSubnetIDs(ec2.FilterForDefaultVPCSubnets)
	if err != nil {
		return nil, fmt.Errorf("get default subnet IDs: %w", err)
	}
	if len(subnets) == 0 {
		return nil, errNoSubnetFound
	}

	arns, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          r.Count,
		Subnets:        subnets,
		TaskFamilyName: taskFamilyName(r.GroupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("run task %s: %w", r.GroupName, err)
	}

	return arns, nil
}

func (r *DefaultVPCRunner) validateDependencies() error {
	if r.VPCGetter == nil {
		return errors.New("vpc getter is not set")
	}

	if r.ClusterGetter == nil {
		return errors.New("cluster getter is not set")
	}

	if r.Starter == nil {
		return errors.New("starter is not set")
	}

	return nil
}
