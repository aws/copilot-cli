// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// NetworkConfigRunner runs an Amazon ECS task in the subnets, security groups, and the default cluster.
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
	Starter       Runner

	// Must not be nil if using default subnets.
	VPCGetter VPCGetter
}

// Run runs tasks in the subnets and the security groups, and returns the tasks.
// If subnets are not provided, it uses the default subnets.
func (r *NetworkConfigRunner) Run() ([]*Task, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.ClusterGetter.DefaultCluster()
	if err != nil {
		return nil, &errGetDefaultCluster{
			parentErr: err,
		}
	}

	if r.Subnets == nil {
		subnets, err := r.VPCGetter.SubnetIDs(ec2.FilterForDefaultVPCSubnets)
		if err != nil {
			return nil, fmt.Errorf(fmtErrDefaultSubnets, err)
		}
		if len(subnets) == 0 {
			return nil, errNoSubnetFound
		}

		r.Subnets = subnets
	}

	ecsTasks, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          r.Count,
		Subnets:        r.Subnets,
		SecurityGroups: r.SecurityGroups,
		TaskFamilyName: taskFamilyName(r.GroupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, &errRunTask{
			groupName: r.GroupName,
			parentErr: err,
		}
	}

	return convertECSTasks(ecsTasks)
}

func (r *NetworkConfigRunner) validateDependencies() error {
	if r.ClusterGetter == nil {
		return errClusterGetterNil
	}

	if r.Starter == nil {
		return errStarterNil
	}

	return nil
}
