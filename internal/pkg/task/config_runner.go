// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"strings"

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
	var essentialContainers []string
	taskDefName := fmt.Sprintf("copilot-%s", r.GroupName)
	taskDefinition, err := r.Starter.TaskDefinition(taskDefName)
	if err != nil {
		return fmt.Errorf("get task definition %s of a task", taskDefName)
	}

	for _, container := range taskDefinition.ContainerDefinitions {
		if aws.BoolValue(container.Essential) {
			essentialContainers = append(essentialContainers, aws.StringValue(container.Name))
		}
	}

	taskARNs := make([]string, len(tasks))
	for idx, task := range tasks {
		taskARNs[idx] = task.TaskARN
	}
	describedTasks, describeErr := r.Starter.DescribeTasks(r.Cluster, taskARNs)
	if describeErr != nil {
		return describeErr
	}
	// Today we only use one container inside a stand-alone task, shall we keep the looping logic
	// active for multiple containers or only add this logic when we have multiple containers inside a stand-alone task in the future.
	for _, describedTask := range describedTasks {
		for _, container := range describedTask.Containers {
			if contains(essentialContainers, aws.StringValue(container.Name)) && aws.Int64Value(container.ExitCode) > 0 {
				taskID, err := ecs.TaskID(aws.StringValue(describedTask.TaskArn))
				if err != nil {
					return err
				}
				return fmt.Errorf(HumanString(aws.StringValue(container.Name),
					taskID,
					aws.Int64Value(container.ExitCode)))
			}
		}
	}
	return nil
}

// contains checks if a string is present in a slice
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// HumanString returns a friendly colorized summary of the container description.
func HumanString(containerName string, taskName string, exitCode int64) string {
	buf := new(strings.Builder)
	code := fmt.Sprintf("%d", exitCode)
	if exitCode > 0 {
		code = color.Red.Sprint(exitCode)
	}
	buf.WriteString(fmt.Sprintf("Container %s in task %s exited with status code %s", containerName, taskName, code))
	return buf.String()
}
