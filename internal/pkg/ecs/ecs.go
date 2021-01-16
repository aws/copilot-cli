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
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	fmtWorkloadTaskDefinitionFamily = "%s-%s-%s"
	fmtTaskTaskDefinitionFamily     = "copilot-%s"
	clusterResourceType             = "ecs:cluster"
	serviceResourceType             = "ecs:service"
)

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

type ecsClient interface {
	RunningTasksInFamily(cluster, family string) ([]*ecs.Task, error)
	RunningTasks(cluster string) ([]*ecs.Task, error)
	ServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error)
	DefaultCluster() (string, error)
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

// ServiceARN returns the ARN of an ECS service created with Copilot.
func (c Client) ServiceARN(app, env, svc string) (*ecs.ServiceArn, error) {
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

// ListActiveWorkloadTasks lists all active workload tasks (with desired status to be RUNNING) in the environment.
func (c Client) ListActiveWorkloadTasks(app, env, workload string) (clusterARN string, taskARNs []string, err error) {
	clusterARN, err = c.ClusterARN(app, env)
	if err != nil {
		return "", nil, fmt.Errorf("get cluster for env %s: %w", env, err)
	}
	tdFamilyName := fmt.Sprintf(fmtWorkloadTaskDefinitionFamily, app, env, workload)
	tasks, err := c.ecsClient.RunningTasksInFamily(clusterARN, tdFamilyName)
	if err != nil {
		return "", nil, fmt.Errorf("list tasks that belong to family %s: %w", tdFamilyName, err)
	}
	for _, task := range tasks {
		taskARNs = append(taskARNs, *task.TaskArn)
	}
	return
}

// ListActiveAppEnvTasksOpts contains the parameters for ListActiveAppEnvTasks.
type ListActiveAppEnvTasksOpts struct {
	App string
	Env string
	ListTasksFilter
}

// ListTasksFilter contains the filtering parameters for listing Copilot tasks.
type ListTasksFilter struct {
	TaskGroup string
	TaskID    string
}

type listActiveCopilotTasksOpts struct {
	Cluster string
	ListTasksFilter
}

// ListActiveAppEnvTasks returns the active Copilot tasks in the environment of an application.
func (c Client) ListActiveAppEnvTasks(opts ListActiveAppEnvTasksOpts) ([]*ecs.Task, error) {
	clusterARN, err := c.ClusterARN(opts.App, opts.Env)
	if err != nil {
		return nil, fmt.Errorf("get cluster for env %s: %w", opts.Env, err)
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

func (c Client) listActiveCopilotTasks(opts listActiveCopilotTasksOpts) ([]*ecs.Task, error) {
	var tasks []*ecs.Task
	if opts.TaskGroup != "" {
		tdFamilyName := fmt.Sprintf(fmtTaskTaskDefinitionFamily, opts.TaskGroup)
		resp, err := c.ecsClient.RunningTasksInFamily(opts.Cluster, tdFamilyName)
		if err != nil {
			return nil, fmt.Errorf("list running tasks that belong to family %s in cluster %s: %w", tdFamilyName, opts.Cluster, err)
		}
		tasks = resp
	} else {
		resp, err := c.ecsClient.RunningTasks(opts.Cluster)
		if err != nil {
			return nil, fmt.Errorf("list running tasks in cluster %s: %w", opts.Cluster, err)
		}
		tasks = resp
	}
	return filterCopilotTask(tasks, opts.TaskID), nil
}

func filterCopilotTask(tasks []*ecs.Task, taskID string) []*ecs.Task {
	var filteredTasks []*ecs.Task

	for _, task := range tasks {
		log.Infoln("")
		log.Infoln(fmt.Sprintf("%v", task))
		var copilotTask bool
		for _, tag := range task.Tags {
			if aws.StringValue(tag.Key) == deploy.TaskTagKey {
				copilotTask = true
				break
			}
		}
		id, _ := ecs.TaskID(aws.StringValue(task.TaskArn))
		log.Infoln(fmt.Sprintf("%v", copilotTask))
		log.Infoln(aws.StringValue(task.TaskArn))
		if copilotTask && strings.Contains(id, taskID) {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}
