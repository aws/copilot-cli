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

const clusterStatusActive = "ACTIVE"

type api interface {
	DescribeClusters(input *ecs.DescribeClustersInput) (*ecs.DescribeClustersOutput, error)
	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	DescribeTasks(input *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
	StopTask(input *ecs.StopTaskInput) (*ecs.StopTaskOutput, error)
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

// ServiceTasks calls ECS API and returns ECS tasks running by a service.
func (e *ECS) ServiceTasks(cluster, service string) ([]*Task, error) {
	return e.listTasks(cluster, withService(service))
}

// RunningTasksInFamily calls ECS API and returns ECS tasks with the desired status to be RUNNING
// within the same task definition family.
func (e *ECS) RunningTasksInFamily(cluster, family string) ([]*Task, error) {
	return e.listTasks(cluster, withFamily(family), withRunningTasks())
}

func (e *ECS) RunningTasks(cluster string) ([]*Task, error) {
	return e.listTasks(cluster, withRunningTasks())
}

type listTasksOpts func(*ecs.ListTasksInput)

func withService(svcName string) listTasksOpts {
	return func(in *ecs.ListTasksInput) {
		in.ServiceName = aws.String(svcName)
	}
}

func withFamily(family string) listTasksOpts {
	return func(in *ecs.ListTasksInput) {
		in.Family = aws.String(family)
	}
}

func withRunningTasks() listTasksOpts {
	return func(in *ecs.ListTasksInput) {
		in.DesiredStatus = aws.String(ecs.DesiredStatusRunning)
	}
}

func (e *ECS) listTasks(cluster string, opts ...listTasksOpts) ([]*Task, error) {
	var tasks []*Task
	in := &ecs.ListTasksInput{
		Cluster: aws.String(cluster),
	}
	for _, opt := range opts {
		opt(in)
	}
	for {
		listTaskResp, err := e.client.ListTasks(in)
		if err != nil {
			return nil, fmt.Errorf("list running tasks: %w", err)
		}
		if len(listTaskResp.TaskArns) == 0 {
			return tasks, nil
		}
		descTaskResp, err := e.client.DescribeTasks(&ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   listTaskResp.TaskArns,
			Include: aws.StringSlice([]string{ecs.TaskFieldTags}),
		})
		if err != nil {
			return nil, fmt.Errorf("describe running tasks in cluster %s: %w", cluster, err)
		}
		for _, task := range descTaskResp.Tasks {
			t := Task(*task)
			tasks = append(tasks, &t)
		}
		if listTaskResp.NextToken == nil {
			break
		}
		in.NextToken = listTaskResp.NextToken
	}
	return tasks, nil
}

// StopTasksOpts sets the optional parameter for StopTasks.
type StopTasksOpts func(*ecs.StopTaskInput)

// WithStopTaskReason sets an optional message specified when a task is stopped.
func WithStopTaskReason(reason string) StopTasksOpts {
	return func(in *ecs.StopTaskInput) {
		in.Reason = aws.String(reason)
	}
}

// WithStopTaskCluster sets the cluster that hosts the task to stop.
func WithStopTaskCluster(cluster string) StopTasksOpts {
	return func(in *ecs.StopTaskInput) {
		in.Cluster = aws.String(cluster)
	}
}

// StopTasks stops multiple running tasks given their IDs or ARNs.
func (e *ECS) StopTasks(tasks []string, opts ...StopTasksOpts) error {
	in := &ecs.StopTaskInput{}
	for _, opt := range opts {
		opt(in)
	}
	for _, task := range tasks {
		in.Task = aws.String(task)
		if _, err := e.client.StopTask(in); err != nil {
			return fmt.Errorf("stop task %s: %w", task, err)
		}
	}
	return nil
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
	if aws.StringValue(cluster.Status) != clusterStatusActive {
		return "", ErrNoDefaultCluster
	}

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
		PropagateTags: aws.String(ecs.PropagateTagsTaskDefinition),
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
		Include: aws.StringSlice([]string{ecs.TaskFieldTags}),
	})

	if waitErr != nil && !isRequestTimeoutErr(waitErr) {
		return nil, fmt.Errorf("wait for tasks to be running: %w", waitErr)
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
		Include: aws.StringSlice([]string{ecs.TaskFieldTags}),
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
