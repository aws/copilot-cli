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

// ConfigRunner runs an Amazon ECS task in the subnets, security groups, and cluster.
// It uses the default subnets and the default cluster if the corresponding field is empty.
type ConfigRunner struct {
	// Count of the tasks to be launched.
	Count int
	// Group Name of the tasks that use the same task definition.
	GroupName string

	// The ARN of the cluster to run the task.
	Cluster string

	// Network configuration
	Subnets        []string
	SecurityGroups []string

	// Interfaces to interact with dependencies. Must not be nil.
	ClusterGetter DefaultClusterGetter
	Starter       Runner

	// Must not be nil if using default subnets.
	VPCGetter VPCGetter

	// Figures non-zero exit code of the task.
	NonZeroExitCodeGetter NonZeroExitCodeGetter

	// Platform configuration
	OS string
}

// Run runs tasks given subnets, security groups and the cluster, and returns the tasks.
// If subnets are not provided, it uses the default subnets.
// If cluster is not provided, it uses the default cluster.
func (r *ConfigRunner) Run() ([]*Task, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	if r.Cluster == "" {
		cluster, err := r.ClusterGetter.DefaultCluster()
		if err != nil {
			return nil, &errGetDefaultCluster{
				parentErr: err,
			}
		}
		r.Cluster = cluster
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
	platformVersion := "LATEST"
	if IsValidWindowsOS(r.OS) {
		platformVersion = "1.0.0"
	}

	ecsTasks, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:         r.Cluster,
		Count:           r.Count,
		Subnets:         r.Subnets,
		SecurityGroups:  r.SecurityGroups,
		TaskFamilyName:  taskFamilyName(r.GroupName),
		StartedBy:       startedBy,
		PlatformVersion: platformVersion,
		EnableExec:      true,
	})
	if err != nil {
		return nil, &errRunTask{
			groupName: r.GroupName,
			parentErr: err,
		}
	}

	return convertECSTasks(ecsTasks), nil
}

func (r *ConfigRunner) validateDependencies() error {
	if r.ClusterGetter == nil {
		return errClusterGetterNil
	}

	if r.Starter == nil {
		return errStarterNil
	}

	return nil
}

// CheckNonZeroExitCode returns the status of the containers part of the given tasks.
func (r *ConfigRunner) CheckNonZeroExitCode(tasks []*Task) error {
	taskARNs := make([]string, len(tasks))
	for idx, task := range tasks {
		taskARNs[idx] = task.TaskARN
	}
	return r.NonZeroExitCodeGetter.HasNonZeroExitCode(taskARNs, r.Cluster)
}
