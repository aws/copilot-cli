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

// ConfigStoreSvc wraps methods of config store.
type ConfigStoreSvc interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListServices(appName string) ([]*config.Service, error)
}

// DeployedEnvServicesLister wraps methods of deploy store.
type DeployedEnvServicesLister interface {
	ListEnvironmentsDeployedTo(appName string, svcName string) ([]string, error)
	ListDeployedServices(appName string, envName string) ([]string, error)
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

// NewServiceConfig contains fields that initiates ServiceDescriber struct.
type NewServiceConfig struct {
	App         string
	Env         string
	Svc         string
	ConfigStore ConfigStoreSvc
}

// NewServiceDescriber instantiates a new service.
func NewServiceDescriber(opt NewServiceConfig) (*ServiceDescriber, error) {
	environment, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", opt.Env, err)
	}
	sess, err := session.NewProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	d := newStackDescriber(sess)
	return &ServiceDescriber{
		app:     opt.App,
		service: opt.Svc,
		env:     opt.Env,

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

// ServiceStackResources returns the filtered service stack resources created by CloudFormation.
func (d *ServiceDescriber) ServiceStackResources() ([]*cloudformation.StackResource, error) {
	svcResources, err := d.stackDescriber.StackResources(stack.NameForService(d.app, d.env, d.service))
	if err != nil {
		return nil, err
	}
	var resources []*cloudformation.StackResource
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
