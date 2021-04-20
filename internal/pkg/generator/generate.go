// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

// GenerateCommandOpts contains information to generate a task run command.
type GenerateCommandOpts struct {
	networkConfiguration ecs.NetworkConfiguration

	executionRole string
	taskRole      string

	containerInfo

	cluster string
}

type containerInfo struct {
	image      string
	entryPoint []string
	command    []string
	envVars    map[string]string
	secrets    map[string]string
}

func containerInformation(taskDef *ecs.TaskDefinition, containerName string) (*containerInfo, error) {
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

// String stringifies a GenerateCommandOpts.
func (o GenerateCommandOpts) String() string {
	output := []string{"copilot task run"}
	if o.executionRole != "" {
		output = append(output, fmt.Sprintf("--execution-role %s", o.executionRole))
	}

	if o.taskRole != "" {
		output = append(output, fmt.Sprintf("--task-role %s", o.taskRole))
	}

	if o.image != "" {
		output = append(output, fmt.Sprintf("--image %s", o.image))
	}

	if o.entryPoint != nil {
		output = append(output, fmt.Sprintf("--entrypoint %s", fmt.Sprintf("\"%s\"", strings.Join(o.entryPoint, " "))))
	}

	if o.command != nil {
		output = append(output, fmt.Sprintf("--command %s", fmt.Sprintf("\"%s\"", strings.Join(o.command, " "))))
	}

	if o.envVars != nil && len(o.envVars) != 0 {
		output = append(output, fmt.Sprintf("--env-vars %s", printStringToStringMap(o.envVars)))
	}

	if o.secrets != nil && len(o.secrets) != 0 {
		output = append(output, fmt.Sprintf("--secrets %s", printStringToStringMap(o.secrets)))
	}

	if o.networkConfiguration.Subnets != nil && len(o.networkConfiguration.Subnets) != 0 {
		output = append(output, fmt.Sprintf("--subnets %s", strings.Join(o.networkConfiguration.Subnets, ",")))
	}

	if o.networkConfiguration.SecurityGroups != nil && len(o.networkConfiguration.SecurityGroups) != 0 {
		output = append(output, fmt.Sprintf("--security-groups %s", strings.Join(o.networkConfiguration.SecurityGroups, ",")))
	}

	if o.cluster != "" {
		output = append(output, fmt.Sprintf("--cluster %s", o.cluster))
	}

	return strings.Join(output, " \\\n")
}

func printStringToStringMap(m map[string]string) string {
	var output []string
	for k, v := range m {
		output = append(output, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(output, ",")
}
