// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
package cloudformation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
)

const (
	projectTagKey = "ecs-project"
	envTagKey     = "ecs-environment"
)

// CloudFormation wraps the CloudFormationAPI interface
type CloudFormation struct {
	client cloudformationiface.CloudFormationAPI
	box    packd.Box
}

type stackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() []*cloudformation.Parameter
	Tags() []*cloudformation.Tag
}

// New returns a configured CloudFormation client.
func New(sess *session.Session) CloudFormation {
	return CloudFormation{
		client: cloudformation.New(sess),
		box:    templates.Box(),
	}
}

// DeployEnvironment creates the CloudFormation stack for an environment by creating and executing a change set.
//
// If the deployment succeeds, returns nil.
// If the stack already exists, returns a ErrStackAlreadyExists.
// If the change set to create the stack cannot be executed, returns a ErrNotExecutableChangeSet.
// Otherwise, returns a wrapped error.
func (cf CloudFormation) DeployEnvironment(env *deploy.CreateEnvironmentInput) error {
	return cf.deploy(newEnvStackConfig(env, cf.box))
}

func (cf CloudFormation) deploy(stackConfig stackConfiguration) error {
	template, err := stackConfig.Template()
	if err != nil {
		return fmt.Errorf("template creation: %w", err)
	}

	in, err := createChangeSetInput(stackConfig.StackName(), template, withCreateChangeSetType(), withTags(stackConfig.Tags()), withParameters(stackConfig.Parameters()))
	if err != nil {
		return err
	}

	if err := cf.deployChangeSet(in); err != nil {
		if stackExists(err) {
			// Explicitly return a StackAlreadyExists error for the caller to decide if they want to ignore the
			// operation or fail the program.
			return &ErrStackAlreadyExists{
				stackName: stackConfig.StackName(),
				parentErr: err,
			}
		}
		return err
	}
	return nil
}

// WaitForEnvironmentCreation will block until the environment's CloudFormation stack has completed or errored.
// Once the deployment is complete, it read the stack output and create an environment with the resources from
// the stack like ECR Repo.
func (cf CloudFormation) WaitForEnvironmentCreation(env *deploy.CreateEnvironmentInput) (*archer.Environment, error) {
	cfEnv := newEnvStackConfig(env, cf.box)
	deployedStack, err := cf.waitForStackCreation(cfEnv)
	if err != nil {
		return nil, err
	}
	return cfEnv.ToEnv(deployedStack)
}

func (cf CloudFormation) waitForStackCreation(stackConfig stackConfiguration) (*cloudformation.Stack, error) {
	describeStackInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackConfig.StackName()),
	}

	if err := cf.client.WaitUntilStackCreateComplete(describeStackInput); err != nil {
		return nil, fmt.Errorf("failed to create stack %s: %w", stackConfig.StackName(), err)
	}

	describeStackOutput, err := cf.client.DescribeStacks(describeStackInput)
	if err != nil {
		return nil, err
	}

	if len(describeStackOutput.Stacks) == 0 {
		return nil, fmt.Errorf("failed to find a stack named %s after it was created", stackConfig.StackName())
	}

	return describeStackOutput.Stacks[0], nil
}

func (cf CloudFormation) deployChangeSet(in *cloudformation.CreateChangeSetInput) error {
	set, err := cf.createChangeSet(in)
	if err != nil {
		return err
	}
	if err := set.waitForCreation(); err != nil {
		return err
	}
	if err := set.execute(); err != nil {
		return err
	}
	return nil
}

func (cf CloudFormation) createChangeSet(in *cloudformation.CreateChangeSetInput) (*changeSet, error) {
	out, err := cf.client.CreateChangeSet(in)
	if err != nil {
		return nil, fmt.Errorf("failed to create changeSet for stack %s: %w", *in.StackName, err)
	}
	return &changeSet{
		name:    aws.StringValue(out.Id),
		stackID: aws.StringValue(out.StackId),
		c:       cf.client,
	}, nil
}

// stackExists returns true if the underlying error is a stack already exists error.
func stackExists(err error) bool {
	currentErr := err
	for {
		if currentErr == nil {
			break
		}
		if aerr, ok := currentErr.(awserr.Error); ok {
			switch aerr.Code() {
			case "ValidationError":
				// A ValidationError occurs if we tried to create the stack with a change set.
				if strings.Contains(aerr.Message(), "already exists") {
					return true
				}
			case cloudformation.ErrCodeAlreadyExistsException:
				// An AlreadyExists error occurs if we tried to create the stack with the CreateStack API.
				return true
			}
		}
		currentErr = errors.Unwrap(currentErr)
	}
	return false
}
