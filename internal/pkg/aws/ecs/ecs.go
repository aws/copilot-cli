// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs contains utility functions for dealing with ECS repos.
package ecs

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type ecsClient interface {
	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
}

// Service wraps an AWS ECS client.
type Service struct {
	ecs ecsClient
}

// New returns a Service configured against the input session.
func New(s *session.Session) *Service {
	return &Service{
		ecs: ecs.New(s),
	}
}

// TaskDefinition calls ECS API and returns the task definition.
func (s Service) TaskDefinition(taskDefName string) (*TaskDefinition, error) {
	resp, err := s.ecs.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe task definition %s: %w", taskDefName, err)
	}
	td := TaskDefinition(*resp.TaskDefinition)
	return &td, nil
}

// TaskDefinition wraps up ECS TaskDefinition struct.
type TaskDefinition ecs.TaskDefinition

// EnvironmentVariables returns environment variables of the task definition.
func (t *TaskDefinition) EnvironmentVariables() map[string]string {
	envs := make(map[string]string)
	for _, env := range t.ContainerDefinitions[0].Environment {
		envs[aws.StringValue(env.Name)] = aws.StringValue(env.Value)
	}
	return envs
}
