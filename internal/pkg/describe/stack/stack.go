// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	clientsession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

const (
	// Ignored resources
	rulePriorityFunction = "Custom::RulePriorityFunction"
	waitCondition        = "AWS::CloudFormation::WaitCondition"
	waitConditionHandle  = "AWS::CloudFormation::WaitConditionHandle"

	serviceLogicalID = "Service"
)

type ecsService interface {
	TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error)
}

type storeSvc interface {
	archer.EnvironmentGetter
	archer.EnvironmentLister
}

type stackDescriber interface {
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DescribeStackResources(input *cloudformation.DescribeStackResourcesInput) (*cloudformation.DescribeStackResourcesOutput, error)
}

type sessionFromRoleProvider interface {
	FromRole(roleARN string, region string) (*clientsession.Session, error)
}

// Describer retrieves information of a CloudFormation Stack.
type Describer struct {
	project string

	store           storeSvc
	ecsClient       map[string]ecsService
	stackDescribers map[string]stackDescriber
	sessProvider    sessionFromRoleProvider
}

// NewDescriber instantiates a new Describer.
func NewDescriber(projectName string) (*Describer, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	return &Describer{
		project:         projectName,
		store:           svc,
		stackDescribers: make(map[string]stackDescriber),
		ecsClient:       make(map[string]ecsService),
		sessProvider:    session.NewProvider(),
	}, nil
}

// EnvVars returns the environment variables of the task definition.
func (d *Describer) EnvVars(env *archer.Environment, appName string) ([]*EnvVars, error) {
	if _, ok := d.ecsClient[env.ManagerRoleARN]; !ok {
		sess, err := d.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
		}
		d.ecsClient[env.ManagerRoleARN] = ecs.New(sess)
	}
	taskDefName := fmt.Sprintf("%s-%s-%s", d.project, env.Name, appName)
	taskDefinition, err := d.ecsClient[env.ManagerRoleARN].TaskDefinition(taskDefName)
	if err != nil {
		return nil, err
	}
	envVars := taskDefinition.EnvironmentVariables()
	var flatEnvVars []*EnvVars
	for name, value := range envVars {
		flatEnvVars = append(flatEnvVars, &EnvVars{
			Environment: env.Name,
			Name:        name,
			Value:       value,
		})
	}

	return flatEnvVars, nil
}

// GetServiceArn returns the ECS service ARN of the application in an environment.
func (d *Describer) GetServiceArn(envName, appName string) (*ecs.ServiceArn, error) {
	env, err := d.store.GetEnvironment(d.project, envName)
	if err != nil {
		return nil, err
	}
	appResources, err := d.describeStackResources(env.ManagerRoleARN, env.Region, stack.NameForApp(d.project, env.Name, appName))
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

// StackResources returns the physical ID of application stack resources created by CloudFormation.
func (d *Describer) StackResources(envName, appName string) ([]*CfnResource, error) {
	env, err := d.store.GetEnvironment(d.project, envName)
	if err != nil {
		return nil, err
	}

	appResource, err := d.describeStackResources(env.ManagerRoleARN, env.Region, stack.NameForApp(d.project, env.Name, appName))
	if err != nil {
		return nil, err
	}
	var resources []*CfnResource
	// See https://github.com/aws/amazon-ecs-cli-v2/issues/621
	ignoredResources := map[string]bool{
		rulePriorityFunction: true,
		waitCondition:        true,
		waitConditionHandle:  true,
	}
	for _, appResource := range appResource {
		if ignoredResources[aws.StringValue(appResource.ResourceType)] {
			continue
		}
		resources = append(resources, &CfnResource{
			PhysicalID: aws.StringValue(appResource.PhysicalResourceId),
			Type:       aws.StringValue(appResource.ResourceType),
		})
	}

	return resources, nil
}

// EnvOutputs returns the output of the environment stack.
func (d *Describer) EnvOutputs(env *archer.Environment) (map[string]string, error) {
	envStack, err := d.stack(env.ManagerRoleARN, env.Region, stack.NameForEnv(d.project, env.Name))
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]string)
	for _, out := range envStack.Outputs {
		outputs[*out.OutputKey] = *out.OutputValue
	}
	return outputs, nil
}

// AppParams returns the parameters of the application stack.
func (d *Describer) AppParams(env *archer.Environment, appName string) (map[string]string, error) {
	appStack, err := d.stack(env.ManagerRoleARN, env.Region, stack.NameForApp(d.project, env.Name, appName))
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, param := range appStack.Parameters {
		params[*param.ParameterKey] = *param.ParameterValue
	}
	return params, nil
}

func (d *Describer) stack(roleARN, region, stackName string) (*cloudformation.Stack, error) {
	svc, err := d.stackDescriber(roleARN, region)
	if err != nil {
		return nil, err
	}
	out, err := svc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe stack %s: %w", stackName, err)
	}
	if len(out.Stacks) == 0 {
		return nil, fmt.Errorf("stack %s not found", stackName)
	}
	return out.Stacks[0], nil
}

func (d *Describer) describeStackResources(roleARN, region, stackName string) ([]*cloudformation.StackResource, error) {
	svc, err := d.stackDescriber(roleARN, region)
	if err != nil {
		return nil, err
	}
	out, err := svc.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe resources for stack %s: %w", stackName, err)
	}
	return out.StackResources, nil
}

func (d *Describer) stackDescriber(roleARN, region string) (stackDescriber, error) {
	if _, ok := d.stackDescribers[roleARN]; !ok {
		sess, err := d.sessProvider.FromRole(roleARN, region)
		if err != nil {
			return nil, fmt.Errorf("session for role %s and region %s: %w", roleARN, region, err)
		}
		d.stackDescribers[roleARN] = cloudformation.New(sess)
	}
	return d.stackDescribers[roleARN], nil
}

// EnvVars contains serialized environment variables for an application.
type EnvVars struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

// CfnResource contains application resources created by cloudformation.
type CfnResource struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
}
