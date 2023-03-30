// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	fmtErrSecurityGroupsFromEnv = "get security groups from environment %s: %w"
	fmtErrDescribeEnvironment   = "describe environment %s: %w"
	fmtErrNumSecurityGroups     = "unable to run task with more than 5 security groups: (%d) %s"

	envSecurityGroupCFNLogicalIDTagKey   = "aws:cloudformation:logical-id"
	envSecurityGroupCFNLogicalIDTagValue = "EnvironmentSecurityGroup"
)

// Names for tag filters
var (
	fmtTagFilterForApp = fmt.Sprintf(ec2.FmtTagFilter, deploy.AppTagKey)
	fmtTagFilterForEnv = fmt.Sprintf(ec2.FmtTagFilter, deploy.EnvTagKey)
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

	// Extra security groups to use.
	SecurityGroups []string

	// Platform configuration.
	OS string

	// Interfaces to interact with dependencies. Must not be nil.
	VPCGetter            VPCGetter
	ClusterGetter        ClusterGetter
	Starter              Runner
	EnvironmentDescriber environmentDescriber

	// Figures non-zero exit code of the task
	NonZeroExitCodeGetter NonZeroExitCodeGetter
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
		Name:   fmt.Sprintf(ec2.FmtTagFilter, envSecurityGroupCFNLogicalIDTagKey),
		Values: []string{envSecurityGroupCFNLogicalIDTagValue},
	})...)
	if err != nil {
		return nil, fmt.Errorf(fmtErrSecurityGroupsFromEnv, r.Env, err)
	}
	securityGroups = appendUniqueStrings(securityGroups, r.SecurityGroups...)
	if numSGs := len(securityGroups); numSGs > 5 {
		return nil, fmt.Errorf(fmtErrNumSecurityGroups, numSGs, strings.Join(securityGroups, ","))
	}

	platformVersion := "LATEST"
	if IsValidWindowsOS(r.OS) {
		platformVersion = "1.0.0"
	}

	ecsTasks, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:         cluster,
		Count:           r.Count,
		Subnets:         subnets,
		SecurityGroups:  securityGroups,
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

func (r *EnvRunner) filtersForVPCFromAppEnv() []ec2.Filter {
	return []ec2.Filter{
		{
			Name:   fmtTagFilterForEnv,
			Values: []string{r.Env},
		},
		{
			Name:   fmtTagFilterForApp,
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

func appendUniqueStrings(s1 []string, s2 ...string) []string {
	for _, v := range s2 {
		if !containsString(s1, v) {
			s1 = append(s1, v)
		}
	}
	return s1
}

func containsString(s []string, search string) bool {
	for _, v := range s {
		if v == search {
			return true
		}
	}
	return false
}

// CheckNonZeroExitCode returns the status of the containers part of the given tasks.
func (r *EnvRunner) CheckNonZeroExitCode(tasks []*Task) error {
	cluster, err := r.ClusterGetter.ClusterARN(r.App, r.Env)
	if err != nil {
		return fmt.Errorf("get cluster for environment %s: %w", r.Env, err)
	}
	taskARNs := make([]string, len(tasks))
	for idx, task := range tasks {
		taskARNs[idx] = task.TaskARN
	}
	return r.NonZeroExitCodeGetter.HasNonZeroExitCode(taskARNs, cluster)
}
