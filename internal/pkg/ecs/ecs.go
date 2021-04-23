// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to retrieve Copilot ECS information.
package ecs

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	fmtWorkloadTaskDefinitionFamily = "%s-%s-%s"
	fmtTaskTaskDefinitionFamily     = "copilot-%s"
	clusterResourceType             = "ecs:cluster"
	serviceResourceType             = "ecs:service"

	taskStopReason = "Task stopped because the underlying CloudFormation stack was deleted."
)

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

type ecsClient interface {
	RunningTasksInFamily(cluster, family string) ([]*ecs.Task, error)
	RunningTasks(cluster string) ([]*ecs.Task, error)
	ServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error)
	DefaultCluster() (string, error)
	StopTasks(tasks []string, opts ...ecs.StopTasksOpts) error
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
	NetworkConfiguration(cluster, serviceName string) (*ecs.NetworkConfiguration, error)
}

// ServiceDesc contains the description of an ECS service.
type ServiceDesc struct {
	Name        string
	ClusterName string
	Tasks       []*ecs.Task
}

// Client retrieves Copilot information from ECS endpoint.
type Client struct {
	rgGetter  resourceGetter
	ecsClient ecsClient
}

// New inits a new Client.
func New(sess *session.Session) *Client {
	return &Client{
		rgGetter:  resourcegroups.New(sess),
		ecsClient: ecs.New(sess),
	}
}

// ClusterARN returns the ARN of the cluster in an environment.
func (c Client) ClusterARN(app, env string) (string, error) {
	return c.clusterARN(app, env)
}

// ServiceARN returns the ARN of an ECS service created with Copilot.
func (c Client) ServiceARN(app, env, svc string) (*ecs.ServiceArn, error) {
	return c.serviceARN(app, env, svc)
}

// DescribeService returns the description of an ECS service given Copilot service info.
func (c Client) DescribeService(app, env, svc string) (*ServiceDesc, error) {
	svcARN, err := c.ServiceARN(app, env, svc)
	if err != nil {
		return nil, err
	}
	clusterName, err := svcARN.ClusterName()
	if err != nil {
		return nil, fmt.Errorf("get cluster name: %w", err)
	}
	serviceName, err := svcARN.ServiceName()
	if err != nil {
		return nil, fmt.Errorf("get service name: %w", err)
	}
	tasks, err := c.ecsClient.ServiceTasks(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get tasks for service %s: %w", serviceName, err)
	}
	return &ServiceDesc{
		ClusterName: clusterName,
		Name:        serviceName,
		Tasks:       tasks,
	}, nil
}

// ListActiveAppEnvTasksOpts contains the parameters for ListActiveAppEnvTasks.
type ListActiveAppEnvTasksOpts struct {
	App string
	Env string
	ListTasksFilter
}

// ListTasksFilter contains the filtering parameters for listing Copilot tasks.
type ListTasksFilter struct {
	TaskGroup   string // Returns only tasks with the given TaskGroup name.
	TaskID      string // Returns only tasks with the given ID.
	CopilotOnly bool   // Returns only tasks with the `copilot-task` tag.
}

type listActiveCopilotTasksOpts struct {
	Cluster string
	ListTasksFilter
}

// ListActiveAppEnvTasks returns the active Copilot tasks in the environment of an application.
func (c Client) ListActiveAppEnvTasks(opts ListActiveAppEnvTasksOpts) ([]*ecs.Task, error) {
	clusterARN, err := c.ClusterARN(opts.App, opts.Env)
	if err != nil {
		return nil, err
	}
	return c.listActiveCopilotTasks(listActiveCopilotTasksOpts{
		Cluster:         clusterARN,
		ListTasksFilter: opts.ListTasksFilter,
	})
}

// ListActiveDefaultClusterTasks returns the active Copilot tasks in the default cluster.
func (c Client) ListActiveDefaultClusterTasks(filter ListTasksFilter) ([]*ecs.Task, error) {
	defaultCluster, err := c.ecsClient.DefaultCluster()
	if err != nil {
		return nil, fmt.Errorf("get default cluster: %w", err)
	}
	return c.listActiveCopilotTasks(listActiveCopilotTasksOpts{
		Cluster:         defaultCluster,
		ListTasksFilter: filter,
	})
}

// StopWorkloadTasks stops all tasks in the given application, enviornment, and workload.
func (c Client) StopWorkloadTasks(app, env, workload string) error {
	return c.stopTasks(app, env, ListTasksFilter{
		TaskGroup: fmt.Sprintf(fmtWorkloadTaskDefinitionFamily, app, env, workload),
	})
}

// StopOneOffTasks stops all one-off tasks in the given application and environment with the family name.
func (c Client) StopOneOffTasks(app, env, family string) error {
	return c.stopTasks(app, env, ListTasksFilter{
		TaskGroup:   fmt.Sprintf(fmtTaskTaskDefinitionFamily, family),
		CopilotOnly: true,
	})
}

// stopTasks stops all tasks in the given application and environment in the given family.
func (c Client) stopTasks(app, env string, filter ListTasksFilter) error {
	tasks, err := c.ListActiveAppEnvTasks(ListActiveAppEnvTasksOpts{
		App:             app,
		Env:             env,
		ListTasksFilter: filter,
	})
	if err != nil {
		return err
	}
	taskIDs := make([]string, len(tasks))
	for n, task := range tasks {
		taskIDs[n] = aws.StringValue(task.TaskArn)
	}
	clusterARN, err := c.ClusterARN(app, env)
	if err != nil {
		return fmt.Errorf("get cluster for env %s: %w", env, err)
	}
	return c.ecsClient.StopTasks(taskIDs, ecs.WithStopTaskCluster(clusterARN), ecs.WithStopTaskReason(taskStopReason))
}

// StopDefaultClusterTasks stops all copilot tasks from the given family in the default cluster.
func (c Client) StopDefaultClusterTasks(familyName string) error {
	tdFamily := fmt.Sprintf(fmtTaskTaskDefinitionFamily, familyName)
	tasks, err := c.ListActiveDefaultClusterTasks(ListTasksFilter{
		TaskGroup:   tdFamily,
		CopilotOnly: true,
	})
	if err != nil {
		return err
	}
	taskIDs := make([]string, len(tasks))
	for n, task := range tasks {
		taskIDs[n] = aws.StringValue(task.TaskArn)
	}
	return c.ecsClient.StopTasks(taskIDs, ecs.WithStopTaskReason(taskStopReason))
}

// TaskDefinition returns the task definition of the service.
func (c Client) TaskDefinition(app, env, svc string) (*ecs.TaskDefinition, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", app, env, svc)
	taskDefinition, err := c.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, fmt.Errorf("get task definition %s of service %s: %w", taskDefName, svc, err)
	}
	return taskDefinition, nil
}

// NetworkConfiguration returns the network configuration of the service.
func (c Client) NetworkConfiguration(app, env, svc string) (*ecs.NetworkConfiguration, error) {
	clusterARN, err := c.clusterARN(app, env)
	if err != nil {
		return nil, err
	}

	arn, err := c.serviceARN(app, env, svc)
	if err != nil {
		return nil, err
	}

	svcName, err := arn.ServiceName()
	if err != nil {
		return nil, fmt.Errorf("extract service name from arn %s: %w", *arn, err)
	}

	return c.ecsClient.NetworkConfiguration(clusterARN, svcName)
}

func (c Client) listActiveCopilotTasks(opts listActiveCopilotTasksOpts) ([]*ecs.Task, error) {
	var tasks []*ecs.Task
	if opts.TaskGroup != "" {
		resp, err := c.ecsClient.RunningTasksInFamily(opts.Cluster, opts.TaskGroup)
		if err != nil {
			return nil, fmt.Errorf("list running tasks in family %s and cluster %s: %w", opts.TaskGroup, opts.Cluster, err)
		}
		tasks = resp
	} else {
		resp, err := c.ecsClient.RunningTasks(opts.Cluster)
		if err != nil {
			return nil, fmt.Errorf("list running tasks in cluster %s: %w", opts.Cluster, err)
		}
		tasks = resp
	}
	if opts.CopilotOnly {
		return filterCopilotTasks(tasks, opts.TaskID), nil
	}
	return filterTasksByID(tasks, opts.TaskID), nil
}

func filterTasksByID(tasks []*ecs.Task, taskID string) []*ecs.Task {
	var filteredTasks []*ecs.Task
	for _, task := range tasks {
		id, _ := ecs.TaskID(aws.StringValue(task.TaskArn))
		if strings.Contains(id, taskID) {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func filterCopilotTasks(tasks []*ecs.Task, taskID string) []*ecs.Task {
	var filteredTasks []*ecs.Task

	for _, task := range filterTasksByID(tasks, taskID) {
		var copilotTask bool
		for _, tag := range task.Tags {
			if aws.StringValue(tag.Key) == deploy.TaskTagKey {
				copilotTask = true
				break
			}
		}
		if copilotTask {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func (c Client) clusterARN(app, env string) (string, error) {
	clusters, err := c.rgGetter.GetResourcesByTags(clusterResourceType, map[string]string{
		deploy.AppTagKey: app,
		deploy.EnvTagKey: env,
	})

	if err != nil {
		return "", fmt.Errorf("get cluster resources for environment %s: %w", env, err)
	}

	if len(clusters) == 0 {
		return "", fmt.Errorf("no cluster found in environment %s", env)
	}

	// NOTE: only one cluster is associated with an application and an environment.
	if len(clusters) > 1 {
		return "", fmt.Errorf("more than one cluster is found in environment %s", env)
	}
	return clusters[0].ARN, nil
}

func (c Client) serviceARN(app, env, svc string) (*ecs.ServiceArn, error) {
	services, err := c.rgGetter.GetResourcesByTags(serviceResourceType, map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: svc,
	})
	if err != nil {
		return nil, fmt.Errorf("get ECS service with tags (%s, %s, %s): %w", app, env, svc, err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no ECS service found for %s in environment %s", svc, env)
	}
	if len(services) > 1 {
		return nil, fmt.Errorf("more than one ECS service with the name %s found in environment %s", svc, env)
	}
	serviceArn := ecs.ServiceArn(services[0].ARN)
	return &serviceArn, nil
}
