// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/dustin/go-humanize"
)

const (
	shortTaskIDLength      = 8
	shortImageDigestLength = 8
	imageDigestPrefix      = "sha256:"
	runTaskStartedBy       = "copilot-task"
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

// TaskDefinition wraps up ECS TaskDefinition struct.
type TaskDefinition ecs.TaskDefinition

// Service wraps up ECS Service struct.
type Service ecs.Service

// Task wraps up ECS Task struct.
type Task ecs.Task

// ServiceStatus contains the status info of a service.
type ServiceStatus struct {
	DesiredCount     int64     `json:"desiredCount"`
	RunningCount     int64     `json:"runningCount"`
	Status           string    `json:"status"`
	LastDeploymentAt time.Time `json:"lastDeploymentAt"`
	TaskDefinition   string    `json:"taskDefinition"`
}

// TaskStatus contains the status info of a task.
type TaskStatus struct {
	Health        string    `json:"health"`
	ID            string    `json:"id"`
	Images        []Image   `json:"images"`
	LastStatus    string    `json:"lastStatus"`
	StartedAt     time.Time `json:"startedAt"`
	StoppedAt     time.Time `json:"stoppedAt"`
	StoppedReason string    `json:"stoppedReason"`
}

// HumanString returns the stringified TaskStatus struct with human readable format.
// Example output:
//   6ca7a60d          f884127d            RUNNING             UNKNOWN             19 hours ago        -
func (t TaskStatus) HumanString() string {
	var digest []string
	imageDigest := "-"
	for _, image := range t.Images {
		if len(image.Digest) < shortImageDigestLength {
			continue
		}
		digest = append(digest, image.Digest[:shortImageDigestLength])
	}
	if len(digest) != 0 {
		imageDigest = strings.Join(digest, ",")
	}
	startedSince := "-"
	if !t.StartedAt.IsZero() {
		startedSince = humanize.Time(t.StartedAt)
	}
	stoppedSince := "-"
	if !t.StoppedAt.IsZero() {
		stoppedSince = humanize.Time(t.StoppedAt)
	}
	shortTaskID := "-"
	if len(t.ID) >= shortTaskIDLength {
		shortTaskID = t.ID[:shortTaskIDLength]
	}
	return fmt.Sprintf("  %s\t%s\t%s\t%s\t%s\t%s\n", shortTaskID, imageDigest, t.LastStatus, t.Health, startedSince, stoppedSince)
}

// Image contains very basic info of a container image.
type Image struct {
	ID     string
	Digest string
}

type RunTaskInput struct {
	Cluster        string
	Count          int64
	Subnets        []string
	SecurityGroups []string
	TaskFamilyName string
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

// DefaultClusters returns the default clusters in the account and region
func (e *ECS) DefaultClusters() ([]string, error) {
	resp, err := e.client.DescribeClusters(&ecs.DescribeClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("get default clusters: %w", err)
	}

	if len(resp.Clusters) == 0 {
		return nil, errors.New("no default cluster is found")
	}

	clusters := make([]string, len(resp.Clusters))
	for idx, cluster := range resp.Clusters {
		clusters[idx] = aws.StringValue(cluster.ClusterArn)
	}

	return clusters, nil
}

// RunTask runs a number of tasks with the task definition and network configurations in a cluster, and returns after
// the task(s) is running or fails to run
func (e *ECS) RunTask(input RunTaskInput) error {
	resp, err := e.client.RunTask(&ecs.RunTaskInput{
		Cluster:        aws.String(input.Cluster),
		Count:          aws.Int64(input.Count),
		LaunchType:     aws.String(ecs.LaunchTypeFargate),
		StartedBy:      aws.String(runTaskStartedBy),
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
		return fmt.Errorf("run task(s) with group name %s: %w", input.TaskFamilyName, err)
	}

	taskArns := make([]string, len(resp.Tasks))
	for idx, task := range resp.Tasks {
		taskArns[idx] = aws.StringValue(task.TaskArn)
	}

	if err := e.client.WaitUntilTasksRunning(&ecs.DescribeTasksInput{
		Cluster: aws.String(input.Cluster),
		Tasks:   aws.StringSlice(taskArns),
	}); err != nil {
		return fmt.Errorf("wait for tasks to be running: %w", err)
	}
	return nil
}

// TaskStatus returns the status of the running task.
func (t *Task) TaskStatus() (*TaskStatus, error) {
	taskID, err := t.taskID(aws.StringValue(t.TaskArn))
	if err != nil {
		return nil, err
	}
	var startedAt, stoppedAt time.Time
	var stoppedReason string

	if t.StoppedAt != nil {
		stoppedAt = *t.StoppedAt
	}
	if t.StartedAt != nil {
		startedAt = *t.StartedAt
	}
	if t.StoppedReason != nil {
		stoppedReason = aws.StringValue(t.StoppedReason)
	}
	var images []Image
	for _, container := range t.Containers {
		images = append(images, Image{
			ID:     aws.StringValue(container.Image),
			Digest: t.imageDigest(aws.StringValue(container.ImageDigest)),
		})
	}
	return &TaskStatus{
		Health:        aws.StringValue(t.HealthStatus),
		ID:            taskID,
		Images:        images,
		LastStatus:    aws.StringValue(t.LastStatus),
		StartedAt:     startedAt,
		StoppedAt:     stoppedAt,
		StoppedReason: stoppedReason,
	}, nil
}

// taskID parses the task ARN and returns the task ID.
// For example: arn:aws:ecs:us-west-2:123456789:task/my-project-test-Cluster-9F7Y0RLP60R7/4082490ee6c245e09d2145010aa1ba8d
// becomes 4082490ee6c245e09d2145010aa1ba8d.
func (t *Task) taskID(taskArn string) (string, error) {
	parsedArn, err := arn.Parse(taskArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	taskID := resources[len(resources)-1]
	return taskID, nil
}

// imageDigest returns the short image digest.
// For example: sha256:18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7
// becomes 18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7.
func (t *Task) imageDigest(imageDigest string) string {
	return strings.TrimPrefix(imageDigest, imageDigestPrefix)
}

// ServiceStatus returns the status of the running service.
func (s *Service) ServiceStatus() ServiceStatus {
	return ServiceStatus{
		Status:           aws.StringValue(s.Status),
		DesiredCount:     aws.Int64Value(s.DesiredCount),
		RunningCount:     aws.Int64Value(s.RunningCount),
		LastDeploymentAt: *s.Deployments[0].UpdatedAt, // FIXME Service assumed to have at least one deployment
		TaskDefinition:   aws.StringValue(s.Deployments[0].TaskDefinition),
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

// ServiceArn is the arn of an ECS service.
type ServiceArn string

// ClusterName returns the cluster name.
// For example: arn:aws:ecs:us-west-2:1234567890:service/my-project-test-Cluster-9F7Y0RLP60R7/my-project-test-myService-JSOH5GYBFAIB
// will return my-project-test-Cluster-9F7Y0RLP60R7
func (s *ServiceArn) ClusterName() (string, error) {
	serviceArn := string(*s)
	parsedArn, err := arn.Parse(serviceArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", serviceArn)
	}
	return resources[1], nil
}

// ServiceName returns the service name.
// For example: arn:aws:ecs:us-west-2:1234567890:service/my-project-test-Cluster-9F7Y0RLP60R7/my-project-test-myService-JSOH5GYBFAIB
// will return my-project-test-myService-JSOH5GYBFAIB
func (s *ServiceArn) ServiceName() (string, error) {
	serviceArn := string(*s)
	parsedArn, err := arn.Parse(serviceArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", serviceArn)
	}
	return resources[2], nil
}
