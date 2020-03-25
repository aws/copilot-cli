// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	shortTaskIDLength      = 7
	shortImageDigestLength = 7
	imageDigestPrefix      = "sha256:"
)

type ecsClient interface {
	DescribeTasks(input *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client ecsClient
}

// TaskDefinition wraps up ECS TaskDefinition struct.
type TaskDefinition ecs.TaskDefinition

// Service wraps up ECS Service struct.
type Service ecs.Service

// Task wraps up ECS Task struct.
type Task ecs.Task

// ServiceStatus contains the status info of a service.
type ServiceStatus struct {
	DesiredCount int64
	RunningCount int64
	Status       string
}

// TaskStatus contains the status info of a task.
type TaskStatus struct {
	DesiredStatus string
	ID            string
	Images        []Image
	LastStatus    string
	StartedAt     int64
	StoppedAt     int64
	StoppedReason string
}

// Image contains very basic info of a container image.
type Image struct {
	ID     string
	Digest string
}

// New returns a Service configured against the input session.
func New(s *session.Session) *ECS {
	return &ECS{
		client: ecs.New(s),
	}
}

// TaskDefinition calls ECS API and returns the task definition.
func (e *ECS) TaskDefinition(taskDefName string) (*TaskDefinition, error) {
	resp, err := e.client.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe task definition %s: %w", taskDefName, err)
	}
	td := TaskDefinition(*resp.TaskDefinition)
	return &td, nil
}

// Service calls ECS API and returns the specified service running in the cluster.
func (e *ECS) Service(clusterName, serviceName string) (*Service, error) {
	resp, err := e.client.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: aws.StringSlice([]string{serviceName}),
	})
	if err != nil {
		return nil, fmt.Errorf("describe service %s: %w", serviceName, err)
	}
	for _, service := range resp.Services {
		if aws.StringValue(service.ServiceName) == serviceName {
			svc := Service(*service)
			return &svc, nil
		}
	}
	return nil, fmt.Errorf("cannot find service %s", serviceName)
}

// Tasks calls ECS API and returns ECS tasks running in the cluster.
func (e *ECS) Tasks(clusterName, serviceName string) ([]*Task, error) {
	var tasks []*Task
	var err error
	listTaskResp := &ecs.ListTasksOutput{}
	for {
		listTaskResp, err = e.client.ListTasks(&ecs.ListTasksInput{
			Cluster:     aws.String(clusterName),
			ServiceName: aws.String(serviceName),
			NextToken:   listTaskResp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list running tasks of service %s: %w", serviceName, err)
		}
		descTaskResp, err := e.client.DescribeTasks(&ecs.DescribeTasksInput{
			Cluster: aws.String(clusterName),
			Tasks:   listTaskResp.TaskArns,
		})
		if err != nil {
			return nil, fmt.Errorf("describe running tasks in cluster %s: %w", clusterName, err)
		}
		for _, task := range descTaskResp.Tasks {
			t := Task(*task)
			tasks = append(tasks, &t)
		}
		if listTaskResp.NextToken == nil {
			break
		}
	}
	return tasks, nil
}

// TaskStatus returns the status of the running task.
func (t *Task) TaskStatus() (*TaskStatus, error) {
	taskID, err := shortTaskID(aws.StringValue(t.TaskArn))
	if err != nil {
		return nil, err
	}
	var stoppedAt int64
	var stoppedReason string
	if t.StoppedAt != nil {
		stoppedAt = t.StoppedAt.Unix()
	}
	if t.StoppedReason != nil {
		stoppedReason = aws.StringValue(t.StoppedReason)
	}
	var images []Image
	for _, container := range t.Containers {
		images = append(images, Image{
			ID:     aws.StringValue(container.Image),
			Digest: shortImageDigest(aws.StringValue(container.ImageDigest)),
		})
	}
	return &TaskStatus{
		DesiredStatus: aws.StringValue(t.DesiredStatus),
		ID:            taskID,
		Images:        images,
		LastStatus:    aws.StringValue(t.LastStatus),
		StartedAt:     t.StartedAt.Unix(),
		StoppedAt:     stoppedAt,
		StoppedReason: stoppedReason,
	}, nil
}

// ServiceStatus returns the status of the running service.
func (s *Service) ServiceStatus() ServiceStatus {
	return ServiceStatus{
		Status:       aws.StringValue(s.Status),
		DesiredCount: aws.Int64Value(s.DesiredCount),
		RunningCount: aws.Int64Value(s.RunningCount),
	}
}

// EnvironmentVariables returns environment variables of the task definition.
func (t *TaskDefinition) EnvironmentVariables() map[string]string {
	envs := make(map[string]string)
	for _, env := range t.ContainerDefinitions[0].Environment {
		envs[aws.StringValue(env.Name)] = aws.StringValue(env.Value)
	}
	return envs
}

// shortTaskID parses the task ARN and returns the short task ID.
// For example: arn:aws:ecs:us-west-2:123456789:task/my-project-test-Cluster-9F7Y0RLP60R7/4082490ee6c245e09d2145010aa1ba8d
// becomes 4082490.
func shortTaskID(taskArn string) (string, error) {
	parsedArn, err := arn.Parse(taskArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	taskID := resources[len(resources)-1]
	return taskID[:shortTaskIDLength], nil
}

// shortImageDigest returns the short image digest.
// For example: sha256:18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7
// becomes 18f7eb6c.
func shortImageDigest(imageDigest string) string {
	return strings.TrimPrefix(imageDigest, imageDigestPrefix)[:shortImageDigestLength]
}
