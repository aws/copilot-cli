// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
)

type JobCommandGenerator struct {
	App string
	Env string
	Job string

	ECSInformationGetter ecsInformationGetter
}

func (g JobCommandGenerator) Generate() (*GenerateCommandOpts, error) {
	taskDef, err := g.ECSInformationGetter.TaskDefinition(g.App, g.Env, g.Job)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition for job %s: %w", g.Job, err)
	}

	containerName := g.Job // NOTE: refer to workload's CloudFormation template. The container name is set to be the workload's name.
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &GenerateCommandOpts{

		executionRole: aws.StringValue(taskDef.ExecutionRoleArn),
		taskRole:      aws.StringValue(taskDef.TaskRoleArn),

		containerInfo: *containerInfo,
	}, nil
}
