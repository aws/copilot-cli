// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type api interface {
	DescribeTasks(input *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	DescribeClusters(input *ecs.DescribeClustersInput) (*ecs.DescribeClustersOutput, error)
	RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
	WaitUntilTasksRunning(input *ecs.DescribeTasksInput) error
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client api
}

// RunTaskInput holds the fields needed to run tasks.
type RunTaskInput struct {
	Cluster        string
	Count          int
	Subnets        []string
	SecurityGroups []string
	TaskFamilyName string
	StartedBy      string
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

// ServiceTasks calls ECS API and returns ECS tasks running in the cluster.
func (e *ECS) ServiceTasks(clusterName, serviceName string) ([]*Task, error) {
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

// DefaultCluster returns the default cluster ARN in the account and region.
func (e *ECS) DefaultCluster() (string, error) {
	resp, err := e.client.DescribeClusters(&ecs.DescribeClustersInput{})
	if err != nil {
		return "", fmt.Errorf("get default cluster: %w", err)
	}

	if len(resp.Clusters) == 0 {
		return "", ErrNoDefaultCluster
	}

	// NOTE: right now at most 1 default cluster is possible, so cluster[0] must be the default cluster
	cluster := resp.Clusters[0]
	return aws.StringValue(cluster.ClusterArn), nil
}

// HasDefaultCluster tries to find the default cluster and returns true if there is one.
func (e *ECS) HasDefaultCluster() (bool, error) {
	if _, err := e.DefaultCluster(); err != nil {
		if errors.Is(err, ErrNoDefaultCluster) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RunTask runs a number of tasks with the task definition and network configurations in a cluster, and returns after
// the task(s) is running or fails to run, along with task ARNs if possible.
func (e *ECS) RunTask(input RunTaskInput) ([]*Task, error) {
	resp, err := e.client.RunTask(&ecs.RunTaskInput{
		Cluster:        aws.String(input.Cluster),
		Count:          aws.Int64(int64(input.Count)),
		LaunchType:     aws.String(ecs.LaunchTypeFargate),
		StartedBy:      aws.String(input.StartedBy),
		TaskDefinition: aws.String(input.TaskFamilyName),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				AssignPublicIp: aws.String(ecs.AssignPublicIpEnabled),
				Subnets:        aws.StringSlice(input.Subnets),
				SecurityGroups: aws.StringSlice(input.SecurityGroups),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("run task(s) %s: %w", input.TaskFamilyName, err)
	}

	taskARNs := make([]string, len(resp.Tasks))
	for idx, task := range resp.Tasks {
		taskARNs[idx] = aws.StringValue(task.TaskArn)
	}

	waitErr := e.client.WaitUntilTasksRunning(&ecs.DescribeTasksInput{
		Cluster: aws.String(input.Cluster),
		Tasks:   aws.StringSlice(taskARNs),
	})

	if waitErr != nil && !isRequestTimeoutErr(waitErr) {
		return nil, fmt.Errorf("wait for tasks to be running: %w", err)
	}

	tasks, describeErr := e.DescribeTasks(input.Cluster, taskARNs)
	if describeErr != nil {
		return nil, describeErr
	}

	if waitErr != nil {
		return nil, &ErrWaiterResourceNotReadyForTasks{tasks: tasks, awsErrResourceNotReady: waitErr}
	}

	return tasks, nil
}

// DescribeTasks returns the tasks with the taskARNs in the cluster.
func (e *ECS) DescribeTasks(cluster string, taskARNs []string) ([]*Task, error) {
	resp, err := e.client.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   aws.StringSlice(taskARNs),
	})
	if err != nil {
		return nil, fmt.Errorf("describe tasks: %w", err)
	}

	tasks := make([]*Task, len(resp.Tasks))
	for idx, task := range resp.Tasks {
		t := Task(*task)
		tasks[idx] = &t
	}
	return tasks, nil
}

func isRequestTimeoutErr(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		return aerr.Code() == request.WaiterResourceNotReadyErrorCode
	}
	return false
}
