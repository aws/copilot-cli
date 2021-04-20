// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

type ecsInformationGetter interface {
	TaskDefinition(app, env, svc string) (*ecs.TaskDefinition, error)
	NetworkConfiguration(app, env, svc string) (*ecs.NetworkConfiguration, error)
	ClusterARN(app, env string) (string, error)
}

// ServiceCommandGenerator generates task run command given a Copilot service.
type ServiceCommandGenerator struct {
	App     string
	Env     string
	Service string

	ECSInformationGetter ecsInformationGetter
}

// Generate generates a task run command.
func (g ServiceCommandGenerator) Generate() (*GenerateCommandOpts, error) {

	networkConfig, err := g.ECSInformationGetter.NetworkConfiguration(g.App, g.Env, g.Service)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s: %w", g.Service, err)
	}

	cluster, err := g.ECSInformationGetter.ClusterARN(g.App, g.Env)
	if err != nil {
		return nil, fmt.Errorf("retrieve cluster ARN created for environment %s in application %s: %w", g.Env, g.App, err)
	}

	taskDef, err := g.ECSInformationGetter.TaskDefinition(g.App, g.Env, g.Service)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition for service %s: %w", g.Service, err)
	}

	containerName := g.Service // NOTE: refer to workload's CloudFormation template. The container name is set to be the workload's name.
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &GenerateCommandOpts{
		networkConfiguration: *networkConfig,

		executionRole: aws.StringValue(taskDef.ExecutionRoleArn),
		taskRole:      aws.StringValue(taskDef.TaskRoleArn),

		containerInfo: *containerInfo,

		cluster: cluster,
	}, nil
}
