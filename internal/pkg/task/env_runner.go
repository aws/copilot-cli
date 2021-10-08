// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	fmtErrSecurityGroupsFromEnv = "get security groups from environment %s: %w"
	fmtErrDescribeEnvironment   = "describe environment %s: %w"

	envSecurityGroupCFNLogicalIDTagKey   = "aws:cloudformation:logical-id"
	envSecurityGroupCFNLogicalIDTagValue = "EnvironmentSecurityGroup"
)

// Names for tag filters
var (
	tagFilterNameForApp = fmt.Sprintf(ec2.TagFilterName, deploy.AppTagKey)
	tagFilterNameForEnv = fmt.Sprintf(ec2.TagFilterName, deploy.EnvTagKey)
)

// EnvRunner can run an Amazon ECS task in the VPC and the cluster of an environment.
type EnvRunner struct {
	// Count of the tasks to be launched.
	Count int
	// Group Name of the tasks that use the same task definition.
	GroupName string

	// App and Env in which the tasks will be launched.
	App string
	Env string

	// Platform configuration
	OS   string
	Arch string

	// Interfaces to interact with dependencies. Must not be nil.
	VPCGetter            VPCGetter
	ClusterGetter        ClusterGetter
	Starter              Runner
	EnvironmentDescriber EnvironmentDescriber
}

// Run runs tasks in the environment of the application, and returns the tasks.
func (r *EnvRunner) Run() ([]*Task, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.ClusterGetter.ClusterARN(r.App, r.Env)
	if err != nil {
		return nil, fmt.Errorf("get cluster for environment %s: %w", r.Env, err)
	}

	description, err := r.EnvironmentDescriber.Describe()
	if err != nil {
		return nil, fmt.Errorf(fmtErrDescribeEnvironment, r.Env, err)
	}
	if len(description.EnvironmentVPC.PublicSubnetIDs) == 0 {
		return nil, errNoSubnetFound
	}

	subnets := description.EnvironmentVPC.PublicSubnetIDs

	filters := r.filtersForVPCFromAppEnv()
	// Use only environment security group https://github.com/aws/copilot-cli/issues/1882.
	securityGroups, err := r.VPCGetter.SecurityGroups(append(filters, ec2.Filter{
		Name:   fmt.Sprintf(ec2.TagFilterName, envSecurityGroupCFNLogicalIDTagKey),
		Values: []string{envSecurityGroupCFNLogicalIDTagValue},
	})...)
	if err != nil {
		return nil, fmt.Errorf(fmtErrSecurityGroupsFromEnv, r.Env, err)
	}

	platformVersion := "LATEST"
	for _, windowsOS := range validWindowsOSs {
		if r.OS == windowsOS {
			platformVersion = "1.0.0"
		}
	}

	ecsTasks, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:         cluster,
		Count:           r.Count,
		Subnets:         subnets,
		SecurityGroups:  securityGroups,
		TaskFamilyName:  taskFamilyName(r.GroupName),
		StartedBy:       startedBy,
		PlatformVersion: platformVersion,
	})
	if err != nil {
		return nil, &errRunTask{
			groupName: r.GroupName,
			parentErr: err,
		}
	}
	return convertECSTasks(ecsTasks), nil
}

func (r *EnvRunner) filtersForVPCFromAppEnv() []ec2.Filter {
	return []ec2.Filter{
		{
			Name:   tagFilterNameForEnv,
			Values: []string{r.Env},
		},
		{
			Name:   tagFilterNameForApp,
			Values: []string{r.App},
		},
	}
}

func (r *EnvRunner) validateDependencies() error {
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
