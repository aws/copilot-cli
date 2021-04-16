// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"io"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/ecs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

const (
	// Ignored resources
	rulePriorityFunction = "Custom::RulePriorityFunction"
	waitCondition        = "AWS::CloudFormation::WaitCondition"
	waitConditionHandle  = "AWS::CloudFormation::WaitConditionHandle"
)

type ecsClient interface {
	TaskDefinition(taskDefName string) (*awsecs.TaskDefinition, error)
	Service(clusterName, serviceName string) (*awsecs.Service, error)
}

type clusterDescriber interface {
	ClusterARN(app, env string) (string, error)
}

// ConfigStoreSvc wraps methods of config store.
type ConfigStoreSvc interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListServices(appName string) ([]*config.Workload, error)
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
	headers := []string{"Environment", "Tasks", "CPU (vCPU)", "Memory (MiB)", "Port"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, config := range c {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", config.Environment, config.Tasks, cpuToString(config.CPU), config.Memory, config.Port)
	}
}

// ServiceDescriber retrieves information about a service.
type ServiceDescriber struct {
	app     string
	service string
	env     string

	ecsClient        ecsClient
	cfn              cfn
	clusterDescriber clusterDescriber
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
	sess, err := sessions.NewProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	return &ServiceDescriber{
		app:     opt.App,
		service: opt.Svc,
		env:     opt.Env,

		ecsClient:        awsecs.New(sess),
		cfn:              cloudformation.New(sess),
		clusterDescriber: ecs.New(sess),
	}, nil
}

// EnvVars returns the environment variables of the task definition.
func (d *ServiceDescriber) EnvVars() ([]*awsecs.ContainerEnvVar, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", d.app, d.env, d.service)
	taskDefinition, err := d.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, err
	}
	return taskDefinition.EnvironmentVariables(), nil
}

// Secrets returns the secrets of the task definition.
func (d *ServiceDescriber) Secrets() ([]*awsecs.ContainerSecret, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", d.app, d.env, d.service)
	taskDefinition, err := d.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, err
	}
	return taskDefinition.Secrets(), nil
}

// NetworkConfiguration returns the network configuration of the service.
func (d *ServiceDescriber) NetworkConfiguration() (*awsecs.NetworkConfiguration, error) {
	clusterARN, err := d.clusterDescriber.ClusterARN(d.app, d.env)
	if err != nil {
		return nil, fmt.Errorf("get cluster ARN for service %s: %w", d.service, err)
	}

	service, err := d.ecsClient.Service(clusterARN, d.service)
	if err != nil {
		return nil, fmt.Errorf("get service %s running on cluster %s: %w", d.service, clusterARN, err)
	}

	networkConfig := service.NetworkConfiguration
	if networkConfig == nil || networkConfig.AwsvpcConfiguration == nil {
		return nil, fmt.Errorf("cannot find the awsvpc configuration for service %s", d.service)
	}

	return &awsecs.NetworkConfiguration{
		AssignPublicIp: aws.StringValue(networkConfig.AwsvpcConfiguration.AssignPublicIp),
		SecurityGroups: aws.StringValueSlice(networkConfig.AwsvpcConfiguration.SecurityGroups),
		Subnets:        aws.StringValueSlice(networkConfig.AwsvpcConfiguration.Subnets),
	}, nil
}

// ServiceStackResources returns the filtered service stack resources created by CloudFormation.
func (d *ServiceDescriber) ServiceStackResources() ([]*cloudformation.StackResource, error) {
	svcResources, err := d.cfn.StackResources(stack.NameForService(d.app, d.env, d.service))
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
	envStack, err := d.cfn.Describe(stack.NameForEnv(d.app, d.env))
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
	svcStack, err := d.cfn.Describe(stack.NameForService(d.app, d.env, d.service))
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, param := range svcStack.Parameters {
		params[*param.ParameterKey] = *param.ParameterValue
	}
	return params, nil
}

// TaskDefinition holds task definition information of the service, including container-level information.
type TaskDefinition struct {
	Images        []*awsecs.ContainerImage
	ExecutionRole string
	TaskRole      string
	EnvVars       []*awsecs.ContainerEnvVar
	Secrets       []*awsecs.ContainerSecret
	EntryPoints   []*awsecs.ContainerEntrypoint
	Commands      []*awsecs.ContainerCommand
}

// TaskDefinition returns the task definition, including the container-level ones, of the service.
func (d *ServiceDescriber) TaskDefinition() (*TaskDefinition, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", d.app, d.env, d.service)
	taskDefinition, err := d.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, fmt.Errorf("get task definition %s of service %s: %w", taskDefName, d.service, err)
	}

	return &TaskDefinition{
		Images:        taskDefinition.Images(),
		ExecutionRole: aws.StringValue(taskDefinition.ExecutionRoleArn),
		TaskRole:      aws.StringValue(taskDefinition.TaskRoleArn),
		EnvVars:       taskDefinition.EnvironmentVariables(),
		Secrets:       taskDefinition.Secrets(),
		EntryPoints:   taskDefinition.EntryPoints(),
		Commands:      taskDefinition.Commands(),
	}, nil
}
