// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to retrieve Copilot ECS information.
package ecs

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/stepfunctions"
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
	DefaultCluster() (string, error)
	Service(clusterName, serviceName string) (*ecs.Service, error)
	NetworkConfiguration(cluster, serviceName string) (*ecs.NetworkConfiguration, error)
	RunningTasks(cluster string) ([]*ecs.Task, error)
	RunningTasksInFamily(cluster, family string) ([]*ecs.Task, error)
	ServiceRunningTasks(clusterName, serviceName string) ([]*ecs.Task, error)
	StoppedServiceTasks(cluster, service string) ([]*ecs.Task, error)
	StopTasks(tasks []string, opts ...ecs.StopTasksOpts) error
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
	UpdateService(clusterName, serviceName string, opts ...ecs.UpdateServiceOpts) error
	DescribeTasks(cluster string, taskARNs []string) ([]*ecs.Task, error)
	ActiveClusters(arns ...string) ([]string, error)
	ActiveServices(clusterName string, serviceARNs ...string) ([]string, error)
	ListServicesByNamespace(namespace string) ([]string, error)
	Services(cluster string, services ...string) ([]*ecs.Service, error)
}

type stepFunctionsClient interface {
	StateMachineDefinition(stateMachineARN string) (string, error)
}

// EnvVar contains the value of an environment variable
type EnvVar struct {
	Name  string
	Value string
}

// ServiceDesc contains the description of an ECS service.
type ServiceDesc struct {
	Name         string
	ClusterName  string
	Tasks        []*ecs.Task // Tasks is a list of tasks with DesiredStatus being RUNNING.
	StoppedTasks []*ecs.Task
}

// Client retrieves Copilot information from ECS endpoint.
type Client struct {
	rgGetter       resourceGetter
	ecsClient      ecsClient
	StepFuncClient stepFunctionsClient
}

// New creates a new Client.
func New(sess *session.Session) *Client {
	return &Client{
		rgGetter:       resourcegroups.New(sess),
		ecsClient:      ecs.New(sess),
		StepFuncClient: stepfunctions.New(sess),
	}
}

// ClusterARN returns the ARN of the cluster in an environment.
func (c Client) ClusterARN(app, env string) (string, error) {
	return c.clusterARN(app, env)
}

// ForceUpdateService forces a new update for an ECS service given Copilot service info.
func (c Client) ForceUpdateService(app, env, svc string) error {
	clusterName, serviceName, err := c.fetchAndParseServiceARN(app, env, svc)
	if err != nil {
		return err
	}
	return c.ecsClient.UpdateService(clusterName, serviceName, ecs.WithForceUpdate())
}

// DescribeService returns the description of an ECS service given Copilot service info.
func (c Client) DescribeService(app, env, svc string) (*ServiceDesc, error) {
	clusterName, serviceName, err := c.fetchAndParseServiceARN(app, env, svc)
	if err != nil {
		return nil, err
	}
	tasks, err := c.ecsClient.ServiceRunningTasks(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get tasks for service %s: %w", serviceName, err)
	}
	stoppedTasks, err := c.ecsClient.StoppedServiceTasks(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get stopped tasks for service %s: %w", serviceName, err)
	}

	return &ServiceDesc{
		ClusterName:  clusterName,
		Name:         serviceName,
		Tasks:        tasks,
		StoppedTasks: stoppedTasks,
	}, nil
}

// Service returns an ECS service given Copilot service info.
func (c Client) Service(app, env, svc string) (*ecs.Service, error) {
	clusterName, serviceName, err := c.fetchAndParseServiceARN(app, env, svc)
	if err != nil {
		return nil, err
	}
	service, err := c.ecsClient.Service(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get ECS service %s: %w", serviceName, err)
	}
	return service, nil
}

// ServiceConnectServices returns a list of services that are in the same
// service connect namespace as the given service, except for itself.
func (c Client) ServiceConnectServices(app, env, svc string) ([]*ecs.Service, error) {
	s, err := c.Service(app, env, svc)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}
	if len(s.Deployments) == 0 {
		return nil, nil
	}

	arns, err := c.ecsClient.ListServicesByNamespace(aws.StringValue(s.Deployments[0].ServiceConnectConfiguration.Namespace))
	if err != nil {
		return nil, fmt.Errorf("get services in the same namespace: %w", err)
	}

	// remove this service's arn
	arns = slices.DeleteFunc(arns, func(arn string) bool {
		return arn == aws.StringValue(s.ServiceArn)
	})

	svcs, err := c.ecsClient.Services(aws.StringValue(s.ClusterArn), arns...)
	if err != nil {
		return nil, fmt.Errorf("get services: %w", err)
	}
	return svcs, nil
}

// LastUpdatedAt returns the last updated time of the ECS service.
func (c Client) LastUpdatedAt(app, env, svc string) (time.Time, error) {
	detail, err := c.Service(app, env, svc)
	if err != nil {
		return time.Time{}, err
	}
	return detail.LastUpdatedAt(), nil
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
	return c.ecsClient.NetworkConfiguration(clusterARN, arn.ServiceName())
}

// NetworkConfigurationForJob returns the network configuration of the job.
func (c Client) NetworkConfigurationForJob(app, env, job string) (*ecs.NetworkConfiguration, error) {
	jobARN, err := c.stateMachineARN(app, env, job)
	if err != nil {
		return nil, err
	}

	raw, err := c.StepFuncClient.StateMachineDefinition(jobARN)
	if err != nil {
		return nil, fmt.Errorf("get state machine definition for job %s: %w", job, err)
	}

	var config NetworkConfiguration
	err = json.Unmarshal([]byte(raw), &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal state machine definition: %w", err)
	}

	return (*ecs.NetworkConfiguration)(&config), nil
}

// NetworkConfiguration wraps an ecs.NetworkConfiguration struct.
type NetworkConfiguration ecs.NetworkConfiguration

// UnmarshalJSON implements custom logic to unmarshal only the network configuration from a state machine definition.
// Example state machine definition:
//
//	 "Version": "1.0",
//	 "Comment": "Run AWS Fargate task",
//	 "StartAt": "Run Fargate Task",
//	 "States": {
//	   "Run Fargate Task": {
//		 "Type": "Task",
//		 "Resource": "arn:aws:states:::ecs:runTask.sync",
//		 "Parameters": {
//		   "LaunchType": "FARGATE",
//		   "PlatformVersion": "1.4.0",
//		   "Cluster": "cluster",
//		   "TaskDefinition": "def",
//		   "PropagateTags": "TASK_DEFINITION",
//		   "Group.$": "$$.Execution.Name",
//		   "NetworkConfiguration": {
//			 "AwsvpcConfiguration": {
//			   "Subnets": ["sbn-1", "sbn-2"],
//			   "AssignPublicIp": "ENABLED",
//			   "SecurityGroups": ["sg-1", "sg-2"]
//			 }
//		   }
//		 },
//		 "End": true
//	   }
func (n *NetworkConfiguration) UnmarshalJSON(b []byte) error {
	var f interface{}
	err := json.Unmarshal(b, &f)
	if err != nil {
		return err
	}

	states := f.(map[string]interface{})["States"].(map[string]interface{})
	parameters := states["Run Fargate Task"].(map[string]interface{})["Parameters"].(map[string]interface{})
	networkConfig := parameters["NetworkConfiguration"].(map[string]interface{})["AwsvpcConfiguration"].(map[string]interface{})

	var subnets []string
	for _, subnet := range networkConfig["Subnets"].([]interface{}) {
		subnets = append(subnets, subnet.(string))
	}

	var securityGroups []string
	for _, sg := range networkConfig["SecurityGroups"].([]interface{}) {
		securityGroups = append(securityGroups, sg.(string))
	}

	n.Subnets = subnets
	n.SecurityGroups = securityGroups
	n.AssignPublicIp = networkConfig["AssignPublicIp"].(string)
	return nil
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
	tags := tags(map[string]string{
		deploy.AppTagKey: app,
		deploy.EnvTagKey: env,
	})

	clusters, err := c.rgGetter.GetResourcesByTags(clusterResourceType, tags)
	switch {
	case err != nil:
		return "", fmt.Errorf("get ECS cluster with tags %s: %w", tags.String(), err)
	case len(clusters) == 0:
		return "", fmt.Errorf("no ECS cluster found with tags %s", tags.String())
	}

	arns := make([]string, len(clusters))
	for i := range clusters {
		arns[i] = clusters[i].ARN
	}

	active, err := c.ecsClient.ActiveClusters(arns...)
	switch {
	case err != nil:
		return "", fmt.Errorf("check if clusters are active: %w", err)
	case len(active) > 1:
		return "", fmt.Errorf("more than one active ECS cluster are found with tags %s", tags.String())
	}

	return active[0], nil
}

func (c Client) fetchAndParseServiceARN(app, env, svc string) (cluster, service string, err error) {
	svcARN, err := c.serviceARN(app, env, svc)
	if err != nil {
		return "", "", err
	}
	return svcARN.ClusterName(), svcARN.ServiceName(), nil
}

func (c Client) serviceARN(app, env, svc string) (*ecs.ServiceArn, error) {
	tags := tags(map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: svc,
	})
	services, err := c.rgGetter.GetResourcesByTags(serviceResourceType, tags)
	if err != nil {
		return nil, fmt.Errorf("get ECS service with tags %s: %w", tags.String(), err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no ECS service found with tags %s", tags.String())
	}
	arns := make([]string, len(services))
	for i := range services {
		arns[i] = services[i].ARN
	}
	activeCluster, err := c.clusterARN(app, env)
	if err != nil {
		return nil, err
	}
	activeSvcs, err := c.ecsClient.ActiveServices(activeCluster, arns...)
	if err != nil {
		return nil, fmt.Errorf("check if services are active in the cluster %s: %w", activeCluster, err)
	}
	if len(activeSvcs) > 1 {
		return nil, fmt.Errorf("more than one ECS service with tags %s", tags.String())
	}
	if len(activeSvcs) == 0 {
		return nil, fmt.Errorf("no active ECS service found")
	}
	serviceARN, err := ecs.ParseServiceArn(activeSvcs[0])
	if err != nil {
		return nil, fmt.Errorf("parse service arn: %w", err)
	}
	return serviceARN, nil
}

type tags map[string]string

func (tags tags) String() string {
	serialized := make([]string, len(tags))
	var i = 0
	for k, v := range tags {
		serialized[i] = fmt.Sprintf("%q=%q", k, v)
		i += 1
	}
	sort.SliceStable(serialized, func(i, j int) bool { return serialized[i] < serialized[j] })
	return strings.Join(serialized, ",")
}

func (c Client) stateMachineARN(app, env, job string) (string, error) {
	tags := tags(map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: job,
	})
	resources, err := c.rgGetter.GetResourcesByTags(resourcegroups.ResourceTypeStateMachine, tags)
	if err != nil {
		return "", fmt.Errorf("get state machine resource with tags %s: %w", tags.String(), err)
	}

	var stateMachineARN string
	targetName := fmt.Sprintf(fmtStateMachineName, app, env, job)
	for _, r := range resources {
		parsedARN, err := arn.Parse(r.ARN)
		if err != nil {
			continue
		}
		parts := strings.Split(parsedARN.Resource, ":")
		if len(parts) != 2 {
			continue
		}
		if parts[1] == targetName {
			stateMachineARN = r.ARN
			break
		}
	}

	if stateMachineARN == "" {
		return "", fmt.Errorf("state machine for job %s not found", job)
	}
	return stateMachineARN, nil
}

// HasNonZeroExitCode returns an error if at least one of the tasks exited with a non-zero exit code. It assumes that all tasks are built on the same task definition.
func (c Client) HasNonZeroExitCode(taskARNs []string, cluster string) error {
	tasks, err := c.ecsClient.DescribeTasks(cluster, taskARNs)
	if err != nil {
		return fmt.Errorf("describe tasks %s: %w", taskARNs, err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("cannot find tasks %s", strings.Join(taskARNs, ", "))
	}

	taskDefinitonARN := aws.StringValue(tasks[0].TaskDefinitionArn)
	taskDefinition, err := c.ecsClient.TaskDefinition(taskDefinitonARN)
	if err != nil {
		return fmt.Errorf("get task definition %s: %w", taskDefinitonARN, err)
	}

	isContainerEssential := make(map[string]bool)
	for _, container := range taskDefinition.ContainerDefinitions {
		isContainerEssential[aws.StringValue(container.Name)] = aws.BoolValue(container.Essential)
	}

	for _, describedTask := range tasks {
		for _, container := range describedTask.Containers {
			if isContainerEssential[aws.StringValue(container.Name)] && aws.Int64Value(container.ExitCode) != 0 {
				taskID, err := ecs.TaskID(aws.StringValue(describedTask.TaskArn))
				if err != nil {
					return err
				}
				return &ErrExitCode{aws.StringValue(container.Name),
					taskID,
					int(aws.Int64Value(container.ExitCode))}
			}
		}
	}
	return nil
}
