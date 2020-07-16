// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	clusterResourceType  = "ecs:cluster"
	fmtErrNoClusterFound = "no cluster found in env %s"
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

	// Interfaces to interact with dependencies. Must not be nil.
	VPCGetter     VPCGetter
	ClusterGetter ResourceGetter
	Starter       TaskRunner
}

// Run runs tasks in the environment of the application, and returns the task ARNs.
func (r *EnvRunner) Run() ([]string, error) {
	if err := r.validateDependencies(); err != nil {
		return nil, err
	}

	cluster, err := r.cluster(r.App, r.Env)
	if err != nil {
		return nil, err
	}

	filters := r.filtersForVPCFromAppEnv()

	subnets, err := r.VPCGetter.PublicSubnetIDs(filters...)
	if err != nil {
		return nil, fmt.Errorf("get public subnet IDs from environment %s: %w", r.Env, err)
	}
	if len(subnets) == 0 {
		return nil, errNoSubnetFound
	}

	securityGroups, err := r.VPCGetter.SecurityGroups(filters...)
	if err != nil {
		return nil, fmt.Errorf("get security groups from environment %s: %w", r.Env, err)
	}

	arns, err := r.Starter.RunTask(ecs.RunTaskInput{
		Cluster:        cluster,
		Count:          r.Count,
		Subnets:        subnets,
		SecurityGroups: securityGroups,
		TaskFamilyName: taskFamilyName(r.GroupName),
		StartedBy:      startedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("run task %s: %w", r.GroupName, err)
	}
	return arns, nil
}

func (r *EnvRunner) cluster(app, env string) (string, error) {
	clusters, err := r.ClusterGetter.GetResourcesByTags(clusterResourceType, map[string]string{
		deploy.AppTagKey: app,
		deploy.EnvTagKey: env,
	})

	if err != nil {
		return "", fmt.Errorf("get cluster by env %s: %w", env, err)
	}

	if len(clusters) == 0 {
		return "", fmt.Errorf(fmtErrNoClusterFound, env)
	}

	// NOTE: only one cluster is associated with an application and an environment
	return clusters[0].ARN, nil
}

func (r *EnvRunner) filtersForVPCFromAppEnv() []ec2.Filter {
	return []ec2.Filter{
		ec2.Filter{
			Name:   tagFilterNameForEnv,
			Values: []string{r.Env},
		},
		ec2.Filter{
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
