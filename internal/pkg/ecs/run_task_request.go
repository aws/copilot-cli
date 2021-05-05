// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to retrieve Copilot ECS information.
package ecs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

// ECSServiceDescriber provides information on an ECS service.
type ECSServiceDescriber interface {
	Service(clusterName, serviceName string) (*awsecs.Service, error)
	TaskDefinition(taskDefName string) (*awsecs.TaskDefinition, error)
	NetworkConfiguration(cluster, serviceName string) (*awsecs.NetworkConfiguration, error)
}

// ServiceDescriber provides information on a Copilot service.
type ServiceDescriber interface {
	TaskDefinition(app, env, svc string) (*awsecs.TaskDefinition, error)
	NetworkConfiguration(app, env, svc string) (*awsecs.NetworkConfiguration, error)
	ClusterARN(app, env string) (string, error)
}

// RunTaskRequest contains information to generate a task run command.
type RunTaskRequest struct {
	networkConfiguration awsecs.NetworkConfiguration

	executionRole string
	taskRole      string
	cluster       string

	containerInfo
}

type containerInfo struct {
	image      string
	entryPoint []string
	command    []string
	envVars    map[string]string
	secrets    map[string]string
}

// RunTaskRequestFromECSService populates a RunTaskRequest with information from an ECS service.
func RunTaskRequestFromECSService(client ECSServiceDescriber, cluster, service string) (*RunTaskRequest, error) {
	networkConfig, err := client.NetworkConfiguration(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s in cluster %s: %w", service, cluster, err)
	}

	svc, err := client.Service(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve service %s in cluster %s: %w", service, cluster, err)
	}

	taskDefNameOrARN := aws.StringValue(svc.TaskDefinition)
	taskDef, err := client.TaskDefinition(taskDefNameOrARN)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition %s: %w", taskDefNameOrARN, err)
	}

	if len(taskDef.ContainerDefinitions) > 1 {
		return nil, fmt.Errorf("found more than one container in task definition: %s", taskDefNameOrARN)
	}

	containerName := aws.StringValue(taskDef.ContainerDefinitions[0].Name)
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &RunTaskRequest{
		networkConfiguration: *networkConfig,
		executionRole:        aws.StringValue(taskDef.ExecutionRoleArn),
		taskRole:             aws.StringValue(taskDef.TaskRoleArn),
		containerInfo:        *containerInfo,
		cluster:              cluster,
	}, nil
}

// RunTaskRequestFromECSService populates a RunTaskRequest with information from a Copilot service.
func RunTaskRequestFromService(client ServiceDescriber, app, env, svc string) (*RunTaskRequest, error) {
	networkConfig, err := client.NetworkConfiguration(app, env, svc)
	if err != nil {
		return nil, fmt.Errorf("retrieve network configuration for service %s: %w", svc, err)
	}

	cluster, err := client.ClusterARN(app, env)
	if err != nil {
		return nil, fmt.Errorf("retrieve cluster ARN created for environment %s in application %s: %w", env, app, err)
	}

	taskDef, err := client.TaskDefinition(app, env, svc)
	if err != nil {
		return nil, fmt.Errorf("retrieve task definition for service %s: %w", svc, err)
	}

	containerName := svc // NOTE: refer to workload's CloudFormation template. The container name is set to be the workload's name.
	containerInfo, err := containerInformation(taskDef, containerName)
	if err != nil {
		return nil, err
	}

	return &RunTaskRequest{
		networkConfiguration: *networkConfig,
		executionRole:        aws.StringValue(taskDef.ExecutionRoleArn),
		taskRole:             aws.StringValue(taskDef.TaskRoleArn),
		containerInfo:        *containerInfo,
		cluster:              cluster,
	}, nil
}

// String stringifies a RunTaskRequest.
func (r RunTaskRequest) String() string {
	output := []string{"copilot task run"}
	if r.executionRole != "" {
		output = append(output, fmt.Sprintf("--execution-role %s", r.executionRole))
	}

	if r.taskRole != "" {
		output = append(output, fmt.Sprintf("--task-role %s", r.taskRole))
	}

	if r.image != "" {
		output = append(output, fmt.Sprintf("--image %s", r.image))
	}

	if r.entryPoint != nil {
		output = append(output, fmt.Sprintf("--entrypoint %s", fmt.Sprintf("\"%s\"", strings.Join(r.entryPoint, " "))))
	}

	if r.command != nil {
		output = append(output, fmt.Sprintf("--command %s", fmt.Sprintf("\"%s\"", strings.Join(r.command, " "))))
	}

	if r.envVars != nil && len(r.envVars) != 0 {
		output = append(output, fmt.Sprintf("--env-vars %s", fmtStringMapToString(r.envVars)))
	}

	if r.secrets != nil && len(r.secrets) != 0 {
		output = append(output, fmt.Sprintf("--secrets %s", fmtStringMapToString(r.secrets)))
	}

	if r.networkConfiguration.Subnets != nil && len(r.networkConfiguration.Subnets) != 0 {
		output = append(output, fmt.Sprintf("--subnets %s", strings.Join(r.networkConfiguration.Subnets, ",")))
	}

	if r.networkConfiguration.SecurityGroups != nil && len(r.networkConfiguration.SecurityGroups) != 0 {
		output = append(output, fmt.Sprintf("--security-groups %s", strings.Join(r.networkConfiguration.SecurityGroups, ",")))
	}

	if r.cluster != "" {
		output = append(output, fmt.Sprintf("--cluster %s", r.cluster))
	}

	return strings.Join(output, " \\\n")
}

func containerInformation(taskDef *awsecs.TaskDefinition, containerName string) (*containerInfo, error) {
	image, err := taskDef.Image(containerName)
	if err != nil {
		return nil, err
	}

	entrypoint, err := taskDef.EntryPoint(containerName)
	if err != nil {
		return nil, err
	}

	command, err := taskDef.Command(containerName)
	if err != nil {
		return nil, err
	}

	envVars := make(map[string]string)
	for _, envVar := range taskDef.EnvironmentVariables() {
		if envVar.Container == containerName {
			envVars[envVar.Name] = envVar.Value
		}
	}

	secrets := make(map[string]string)
	for _, secret := range taskDef.Secrets() {
		if secret.Container == containerName {
			secrets[secret.Name] = secret.ValueFrom
		}
	}

	return &containerInfo{
		image:      image,
		entryPoint: entrypoint,
		command:    command,
		envVars:    envVars,
		secrets:    secrets,
	}, nil
}

// This function will format a map to a string as "key1=value1,key2=value2,key3=value3".
func fmtStringMapToString(m map[string]string) string {
	var output []string

	// Sort the map so that `output` is consistent and the unit test won't be flaky.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		output = append(output, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return strings.Join(output, ",")
}
