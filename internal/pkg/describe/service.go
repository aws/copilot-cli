// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

const (
	// Ignored resources
	rulePriorityFunction = "Custom::RulePriorityFunction"
	waitCondition        = "AWS::CloudFormation::WaitCondition"
	waitConditionHandle  = "AWS::CloudFormation::WaitConditionHandle"

	serviceLogicalID = "Service"
)

type stackAndResourcesDescriber interface {
	Stack(stackName string) (*cloudformation.Stack, error)
	StackResources(stackName string) ([]*cloudformation.StackResource, error)
}

type ecsClient interface {
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
}

// ServiceConfig contains serialized configuration parameters for a service.
type ServiceConfig struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

type configurations []*ServiceConfig

func (c configurations) humanString(w io.Writer) {
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", "Environment", "Tasks", "CPU (vCPU)", "Memory (MiB)", "Port")
	for _, config := range c {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", config.Environment, config.Tasks, cpuToString(config.CPU), config.Memory, config.Port)
	}
}

// ServiceDescriber retrieves information about a service.
type ServiceDescriber struct {
	app     string
	service string
	env     string

	ecsClient      ecsClient
	stackDescriber stackAndResourcesDescriber
}

// NewServiceDescriber instantiates a new service.
func NewServiceDescriber(app, env, svc string) (*ServiceDescriber, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := store.GetService(app, svc)
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", svc, err)
	}
	environment, err := store.GetEnvironment(app, env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", env, err)
	}
	sess, err := session.NewProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	d := newStackDescriber(sess)
	return &ServiceDescriber{
		app:     app,
		service: meta.Name,
		env:     environment.Name,

		ecsClient:      ecs.New(sess),
		stackDescriber: d,
	}, nil
}

// EnvVars returns the environment variables of the task definition.
func (d *ServiceDescriber) EnvVars() (map[string]string, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", d.app, d.env, d.service)
	taskDefinition, err := d.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, err
	}
	envVars := taskDefinition.EnvironmentVariables()

	return envVars, nil
}

// GetServiceArn returns the ECS service ARN of the service in an environment.
func (d *ServiceDescriber) GetServiceArn() (*ecs.ServiceArn, error) {
	svcResources, err := d.stackDescriber.StackResources(stack.NameForService(d.app, d.env, d.service))
	if err != nil {
		return nil, err
	}
	for _, svcResource := range svcResources {
		if aws.StringValue(svcResource.LogicalResourceId) == serviceLogicalID {
			serviceArn := ecs.ServiceArn(aws.StringValue(svcResource.PhysicalResourceId))
			return &serviceArn, nil
		}
	}
	return nil, fmt.Errorf("cannot find service arn in service stack resource")
}

// ServiceStackResources returns the filtered service stack resources created by CloudFormation.
func (d *ServiceDescriber) ServiceStackResources() ([]*cloudformation.StackResource, error) {
	svcResources, err := d.stackDescriber.StackResources(stack.NameForService(d.app, d.env, d.service))
	if err != nil {
		return nil, err
	}
	var resources []*cloudformation.StackResource
	// TODO: rename this url once repo name changes.
	// See https://github.com/aws/copilot-cli/issues/621
	ignoredResources := map[string]bool{
		rulePriorityFunction: true,
		waitCondition:        true,
		waitConditionHandle:  true,
	}
	for _, svcResource := range svcResources {
		if ignoredResources[aws.StringValue(svcResource.ResourceType)] {
			continue
		}
		resources = append(resources, svcResource)
	}

	return resources, nil
}

// EnvOutputs returns the output of the environment stack.
func (d *ServiceDescriber) EnvOutputs() (map[string]string, error) {
	envStack, err := d.stackDescriber.Stack(stack.NameForEnv(d.app, d.env))
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]string)
	for _, out := range envStack.Outputs {
		outputs[*out.OutputKey] = *out.OutputValue
	}
	return outputs, nil
}

// Params returns the parameters of the service stack.
func (d *ServiceDescriber) Params() (map[string]string, error) {
	svcStack, err := d.stackDescriber.Stack(stack.NameForService(d.app, d.env, d.service))
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, param := range svcStack.Parameters {
		params[*param.ParameterKey] = *param.ParameterValue
	}
	return params, nil
}

func flattenEnvVars(envName string, m map[string]string) []*EnvVars {
	var envVarList []*EnvVars
	for k, v := range m {
		envVarList = append(envVarList, &EnvVars{
			Environment: envName,
			Name:        k,
			Value:       v,
		})
	}
	return envVarList
}
