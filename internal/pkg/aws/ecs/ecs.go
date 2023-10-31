// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
)

const (
	statusActive                     = "ACTIVE"
	waitServiceStablePollingInterval = 15 * time.Second
	waitServiceStableMaxTry          = 80
	stableServiceDeploymentNum       = 1

	// EndpointsID is the ID to look up the ECS service endpoint.
	EndpointsID = ecs.EndpointsID
)

type api interface {
	DescribeClusters(input *ecs.DescribeClustersInput) (*ecs.DescribeClustersOutput, error)
	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	DescribeTasks(input *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
	ExecuteCommand(input *ecs.ExecuteCommandInput) (*ecs.ExecuteCommandOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
	StopTask(input *ecs.StopTaskInput) (*ecs.StopTaskOutput, error)
	UpdateService(input *ecs.UpdateServiceInput) (*ecs.UpdateServiceOutput, error)
	WaitUntilTasksRunning(input *ecs.DescribeTasksInput) error
	ListServicesByNamespacePages(input *ecs.ListServicesByNamespaceInput, fn func(*ecs.ListServicesByNamespaceOutput, bool) bool) error
}

type ssmSessionStarter interface {
	StartSession(ssmSession *ecs.Session) error
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client         api
	newSessStarter func() ssmSessionStarter

	maxServiceStableTries int
	pollIntervalDuration  time.Duration
}

// RunTaskInput holds the fields needed to run tasks.
type RunTaskInput struct {
	Cluster         string
	Count           int
	Subnets         []string
	SecurityGroups  []string
	TaskFamilyName  string
	StartedBy       string
	PlatformVersion string
	EnableExec      bool
}

// ExecuteCommandInput holds the fields needed to execute commands in a running container.
type ExecuteCommandInput struct {
	Cluster   string
	Command   string
	Task      string
	Container string
}

// New returns a Service configured against the input session.
func New(s *session.Session) *ECS {
	return &ECS{
		client: ecs.New(s),
		newSessStarter: func() ssmSessionStarter {
			return exec.NewSSMPluginCommand(s)
		},
		maxServiceStableTries: waitServiceStableMaxTry,
		pollIntervalDuration:  waitServiceStablePollingInterval,
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
	svcs, err := e.Services(clusterName, serviceName)
	if err != nil {
		return nil, err
	}
	if aws.StringValue(svcs[0].ServiceName) != serviceName {
		return nil, fmt.Errorf("cannot find service %s", serviceName)
	}

	return svcs[0], nil
}

// Services calls the ECS API and returns all of the specified services running in cluster.
func (e *ECS) Services(cluster string, services ...string) ([]*Service, error) {
	var svcs []*Service

	for i := 0; i < len(services); i += 10 {
		split := services[i:min(10+i, len(services))]

		resp, err := e.client.DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: aws.StringSlice(split),
		})
		switch {
		case err != nil:
			return nil, fmt.Errorf("describe services: %w", err)
		case len(resp.Failures) > 0:
			return nil, fmt.Errorf("describe services: %s", resp.Failures[0].String())
		case len(resp.Services) != len(split):
			return nil, fmt.Errorf("describe services: got %v services, but expected %v", len(resp.Services), len(split))
		}

		for j := range resp.Services {
			svc := Service(*resp.Services[j])
			svcs = append(svcs, &svc)
		}
	}

	return svcs, nil
}

// ListServicesByNamespace returns a list of service ARNs of services that
// are in the given namespace.
func (e *ECS) ListServicesByNamespace(namespace string) ([]string, error) {
	var arns []string
	err := e.client.ListServicesByNamespacePages(&ecs.ListServicesByNamespaceInput{
		Namespace: aws.String(namespace),
	}, func(resp *ecs.ListServicesByNamespaceOutput, b bool) bool {
		arns = append(arns, aws.StringValueSlice(resp.ServiceArns)...)
		return true
	})
	if err != nil {
		return nil, err
	}
	return arns, nil
}

// UpdateServiceOpts sets the optional parameter for UpdateService.
type UpdateServiceOpts func(*ecs.UpdateServiceInput)

// WithForceUpdate sets ForceNewDeployment to force an update.
func WithForceUpdate() UpdateServiceOpts {
	return func(in *ecs.UpdateServiceInput) {
		in.ForceNewDeployment = aws.Bool(true)
	}
}

// UpdateService calls ECS API and updates the specific service running in the cluster.
func (e *ECS) UpdateService(clusterName, serviceName string, opts ...UpdateServiceOpts) error {
	in := &ecs.UpdateServiceInput{
		Cluster: aws.String(clusterName),
		Service: aws.String(serviceName),
	}
	for _, opt := range opts {
		opt(in)
	}
	svc, err := e.client.UpdateService(in)
	if err != nil {
		return fmt.Errorf("update service %s from cluster %s: %w", serviceName, clusterName, err)
	}
	s := Service(*svc.Service)
	if err := e.waitUntilServiceStable(&s); err != nil {
		return fmt.Errorf("wait until service %s becomes stable: %w", serviceName, err)
	}
	return nil
}

// waitUntilServiceStable waits until the service is stable.
// See https://docs.aws.amazon.com/cli/latest/reference/ecs/wait/services-stable.html
func (e *ECS) waitUntilServiceStable(svc *Service) error {
	var err error
	var tryNum int
	for {
		if len(svc.Deployments) == stableServiceDeploymentNum &&
			aws.Int64Value(svc.DesiredCount) == aws.Int64Value(svc.RunningCount) {
			// This conditional is sufficient to determine that the service is stable AND has successfully updated (hence not rolled-back).
			// The stable service cannot be a rolled-back service because a rollback can only be triggered by circuit breaker after ~1hr,
			// by which time we would have already timed out.
			return nil
		}
		if tryNum >= e.maxServiceStableTries {
			return &ErrWaitServiceStableTimeout{
				maxRetries: e.maxServiceStableTries,
			}
		}
		svc, err = e.Service(aws.StringValue(svc.ClusterArn), aws.StringValue(svc.ServiceName))
		if err != nil {
			return err
		}
		tryNum++
		time.Sleep(e.pollIntervalDuration)
	}
}

// ServiceRunningTasks calls ECS API and returns the ECS tasks spun up by the service, with the desired status to be set to be RUNNING.
func (e *ECS) ServiceRunningTasks(cluster, service string) ([]*Task, error) {
	return e.listTasks(cluster, withService(service), withRunningTasks())
}

// StoppedServiceTasks calls ECS API and returns stopped ECS tasks in a service.
func (e *ECS) StoppedServiceTasks(cluster, service string) ([]*Task, error) {
	return e.listTasks(cluster, withService(service), withStoppedTasks())
}

// RunningTasksInFamily calls ECS API and returns ECS tasks with the desired status to be RUNNING
// within the same task definition family.
func (e *ECS) RunningTasksInFamily(cluster, family string) ([]*Task, error) {
	return e.listTasks(cluster, withFamily(family), withRunningTasks())
}

// RunningTasks calls ECS API and returns ECS tasks with the desired status to be RUNNING.
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

func withStoppedTasks() listTasksOpts {
	return func(in *ecs.ListTasksInput) {
		in.DesiredStatus = aws.String(ecs.DesiredStatusStopped)
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
	if aws.StringValue(cluster.Status) != statusActive {
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

// ActiveClusters returns the subset of cluster arns that have an ACTIVE status.
func (e *ECS) ActiveClusters(arns ...string) ([]string, error) {
	resp, err := e.client.DescribeClusters(&ecs.DescribeClustersInput{
		Clusters: aws.StringSlice(arns),
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("describe clusters: %w", err)
	case len(resp.Failures) > 0:
		return nil, fmt.Errorf("describe clusters: %s", resp.Failures[0].GoString())
	}

	var active []string
	for _, cluster := range resp.Clusters {
		if aws.StringValue(cluster.Status) == statusActive {
			active = append(active, aws.StringValue(cluster.ClusterArn))
		}
	}

	return active, nil
}

// ActiveServices returns the subset of service arns that have an ACTIVE status from the given cluster.
func (e *ECS) ActiveServices(clusterARN string, serviceARNs ...string) ([]string, error) {
	// All the filteredSvcARNs will belong to the given Cluster.
	filteredSvcARNS, err := e.filterServiceARNs(clusterARN, serviceARNs...)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterARN),
		Services: aws.StringSlice(filteredSvcARNS),
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("describe services: %w", err)
	case len(resp.Failures) > 0:
		return nil, fmt.Errorf("describe services: %s", resp.Failures[0].GoString())
	}

	var active []string
	for _, svc := range resp.Services {
		if aws.StringValue(svc.Status) == statusActive {
			active = append(active, aws.StringValue(svc.ServiceArn))
		}
	}

	return active, nil
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
		EnableExecuteCommand: aws.Bool(input.EnableExec),
		PlatformVersion:      aws.String(input.PlatformVersion),
		PropagateTags:        aws.String(ecs.PropagateTagsTaskDefinition),
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

// ExecuteCommand executes commands in a running container, and then terminate the session.
func (e *ECS) ExecuteCommand(in ExecuteCommandInput) (err error) {
	execCmdresp, err := e.client.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     aws.String(in.Cluster),
		Command:     aws.String(in.Command),
		Container:   aws.String(in.Container),
		Interactive: aws.Bool(true),
		Task:        aws.String(in.Task),
	})
	if err != nil {
		return &ErrExecuteCommand{err: err}
	}
	sessID := aws.StringValue(execCmdresp.Session.SessionId)
	if err = e.newSessStarter().StartSession(execCmdresp.Session); err != nil {
		err = fmt.Errorf("start session %s using ssm plugin: %w", sessID, err)
	}
	return err
}

// NetworkConfiguration returns the network configuration of a service.
func (e *ECS) NetworkConfiguration(cluster, serviceName string) (*NetworkConfiguration, error) {
	service, err := e.service(cluster, serviceName)
	if err != nil {
		return nil, err
	}

	networkConfig := service.NetworkConfiguration
	if networkConfig == nil || networkConfig.AwsvpcConfiguration == nil {
		return nil, fmt.Errorf("cannot find the awsvpc configuration for service %s", serviceName)
	}

	return &NetworkConfiguration{
		AssignPublicIp: aws.StringValue(networkConfig.AwsvpcConfiguration.AssignPublicIp),
		SecurityGroups: aws.StringValueSlice(networkConfig.AwsvpcConfiguration.SecurityGroups),
		Subnets:        aws.StringValueSlice(networkConfig.AwsvpcConfiguration.Subnets),
	}, nil
}

func (e *ECS) service(clusterName, serviceName string) (*Service, error) {
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

// filterServiceARNs returns subset of the ServiceARNs that belong to the given Cluster.
func (e *ECS) filterServiceARNs(clusterARN string, serviceARNs ...string) ([]string, error) {
	var filtered []string
	for _, arn := range serviceARNs {
		svcArn, err := ParseServiceArn(arn)
		if err != nil {
			return nil, err
		}
		if svcArn.ClusterArn() == clusterARN {
			filtered = append(filtered, arn)
		}
	}
	return filtered, nil
}

func isRequestTimeoutErr(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		return aerr.Code() == request.WaiterResourceNotReadyErrorCode
	}
	return false
}
