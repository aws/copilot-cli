// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"

	"github.com/aws/aws-sdk-go/aws"
)

type ecsClient interface {
	Service(clusterName, serviceName string) (*ecs.Service, error)
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
	NetworkConfiguration(cluster, serviceName string) (*ecs.NetworkConfiguration, error)
}

// ECSServiceCommandGenerator generates task run command given an ECS service.
type ECSServiceCommandGenerator struct {
	Cluster   string
	Service   string
	ECSClient ecsClient
}

// Generate generates a task run command.
func (g ECSServiceCommandGenerator) Generate() (*GenerateCommandOpts, error) {
	networkConfig, err := g.ECSClient.NetworkConfiguration(g.Cluster, g.Service)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s in cluster %s: %w", g.Service, g.Cluster, err)
	}

	svc, err := g.ECSClient.Service(g.Cluster, g.Service)
	if err != nil {
		return nil, fmt.Errorf("retrieve service %s in cluster %s: %w", g.Service, g.Cluster, err)
	}

	taskDefNameOrARN := aws.StringValue(svc.TaskDefinition)
	taskDef, err := g.ECSClient.TaskDefinition(taskDefNameOrARN)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition %s: %w", taskDefNameOrARN, err)
	}

	if len(taskDef.ContainerDefinitions) > 1 {
		return nil, fmt.Errorf("found more than one container in task definition: %s", taskDefNameOrARN)
	}

	containerName := aws.StringValue(taskDef.ContainerDefinitions[0].Name)
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &GenerateCommandOpts{
		networkConfiguration: *networkConfig,
		executionRole:        aws.StringValue(taskDef.ExecutionRoleArn),
		taskRole:             aws.StringValue(taskDef.TaskRoleArn),
		containerInfo:        *containerInfo,
		cluster:              g.Cluster,
	}, nil
}
