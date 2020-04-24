// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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

type ecsService interface {
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
}

// AppDescriber retrieves information about an application.
type AppDescriber struct {
	project string
	app     string
	env     string

	ecsClient      ecsService
	stackDescriber stackAndResourcesDescriber
}

// NewAppDescriber instantiates a new application.
func NewAppDescriber(project, env, app string) (*AppDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetApplication(project, app)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", app, err)
	}
	environment, err := svc.GetEnvironment(project, env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", env, err)
	}
	sess, err := session.NewProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	d := newStackDescriber(sess)
	return &AppDescriber{
		project: project,
		app:     meta.Name,
		env:     environment.Name,

		ecsClient:      ecs.New(sess),
		stackDescriber: d,
	}, nil
}

// EnvVars returns the environment variables of the task definition.
func (d *AppDescriber) EnvVars() (map[string]string, error) {
	taskDefName := fmt.Sprintf("%s-%s-%s", d.project, d.env, d.app)
	taskDefinition, err := d.ecsClient.TaskDefinition(taskDefName)
	if err != nil {
		return nil, err
	}
	envVars := taskDefinition.EnvironmentVariables()

	return envVars, nil
}

// GetServiceArn returns the ECS service ARN of the application in an environment.
func (d *AppDescriber) GetServiceArn() (*ecs.ServiceArn, error) {
	appResources, err := d.stackDescriber.StackResources(stack.NameForApp(d.project, d.env, d.app))
	if err != nil {
		return nil, err
	}
	for _, appResource := range appResources {
		if aws.StringValue(appResource.LogicalResourceId) == serviceLogicalID {
			serviceArn := ecs.ServiceArn(aws.StringValue(appResource.PhysicalResourceId))
			return &serviceArn, nil
		}
	}
	return nil, fmt.Errorf("cannot find service arn in app stack resource")
}

// AppStackResources returns the filtered application stack resources created by CloudFormation.
func (d *AppDescriber) AppStackResources() ([]*cloudformation.StackResource, error) {
	appResources, err := d.stackDescriber.StackResources(stack.NameForApp(d.project, d.env, d.app))
	if err != nil {
		return nil, err
	}
	var resources []*cloudformation.StackResource
	// See https://github.com/aws/amazon-ecs-cli-v2/issues/621
	ignoredResources := map[string]bool{
		rulePriorityFunction: true,
		waitCondition:        true,
		waitConditionHandle:  true,
	}
	for _, appResource := range appResources {
		if ignoredResources[aws.StringValue(appResource.ResourceType)] {
			continue
		}
		resources = append(resources, appResource)
	}

	return resources, nil
}

// EnvOutputs returns the output of the environment stack.
func (d *AppDescriber) EnvOutputs() (map[string]string, error) {
	envStack, err := d.stackDescriber.Stack(stack.NameForEnv(d.project, d.env))
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]string)
	for _, out := range envStack.Outputs {
		outputs[*out.OutputKey] = *out.OutputValue
	}
	return outputs, nil
}

// Params returns the parameters of the application stack.
func (d *AppDescriber) Params() (map[string]string, error) {
	appStack, err := d.stackDescriber.Stack(stack.NameForApp(d.project, d.env, d.app))
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, param := range appStack.Parameters {
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

func flattenResources(stackResources []*cloudformation.StackResource) []*CfnResource {
	var webAppResources []*CfnResource
	for _, stackResource := range stackResources {
		webAppResources = append(webAppResources, &CfnResource{
			PhysicalID: aws.StringValue(stackResource.PhysicalResourceId),
			Type:       aws.StringValue(stackResource.ResourceType),
		})
	}
	return webAppResources
}
