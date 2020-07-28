// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

const (
	fmtErrDefaultSubnets = "get default subnet IDs: %w"
)

// DefaultVPCRunner can run an Amazon ECS task in the default VPC and the default cluster.
type DefaultVPCRunner struct {
	// Count of the tasks to be launched.
	Count int
	// Group Name fo the tasks that use the same task definition.
	GroupName string

	// Interfaces to interact with dependencies. Must not be nil.
	VPCGetter     VPCGetter
	ClusterGetter DefaultClusterGetter
	Starter       TaskRunner
}

// Run runs tasks in default cluster and default subnets, and returns the tasks.
func (r *DefaultVPCRunner) Run() ([]*Task, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.ClusterGetter.DefaultCluster()
	if err != nil {
		return nil, &errGetDefaultCluster{
			parentErr: err,
		}
	}

	subnets, err := r.VPCGetter.SubnetIDs(ec2.FilterForDefaultVPCSubnets)
	if err != nil {
		return nil, fmt.Errorf(fmtErrDefaultSubnets, err)
	}
	if len(subnets) == 0 {
		return nil, errNoSubnetFound
	}

	ecsTasks, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          r.Count,
		Subnets:        subnets,
		TaskFamilyName: taskFamilyName(r.GroupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, &errRunTask{
			groupName: r.GroupName,
			parentErr: err,
		}
	}

	return convertECSTasks(ecsTasks), nil
}

func (r *DefaultVPCRunner) validateDependencies() error {
	if r.VPCGetter == nil {
		return errVPCGetterNil
	}

	if r.ClusterGetter == nil {
		return errClusterGetterNil
	}

	if r.Starter == nil {
		return errStarterNil
	}

	return nil
}
