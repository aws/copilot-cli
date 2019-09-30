// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
package cloudformation

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2"
)

const (
	cloudformationTemplatesPath = "../../../../templates/cloudformation"
	environmentTemplate         = "environment.yml"
	includeLoadBalancerParamKey = "IncludePublicLoadBalancer"
)

// CloudFormation wraps the CloudFormationAPI interface
type CloudFormation struct {
	client cloudformationiface.CloudFormationAPI
	box    packd.Box
}

// New returns a configured CloudFormation client.
func New(sess *session.Session) CloudFormation {
	return CloudFormation{
		client: cloudformation.New(sess),
		box:    packr.New("cloudformation", cloudformationTemplatesPath),
	}
}

// DeployEnvironment creates the CloudFormation stack for an environment by creating and executing a change set.
//
// If the deployment succeeds, returns nil.
// If the stack already exists, returns a ErrStackAlreadyExists.
// Otherwise, returns a wrapped error.
func (cf CloudFormation) DeployEnvironment(env *archer.Environment) error {
	template, err := cf.box.FindString(environmentTemplate)
	if err != nil {
		return fmt.Errorf("failed to find template %s for the environment: %w", environmentTemplate, err)
	}

	in, err := createChangeSetInput(envStackName(env), template, withCreateChangeSetType(), withParameters([]*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(includeLoadBalancerParamKey),
			ParameterValue: aws.String(strconv.FormatBool(env.PublicLoadBalancer)),
		},
	}))
	if err != nil {
		return err
	}

	if err := cf.deployChangeSet(in); err != nil {
		if stackExists(err) {
			// Explicitly return a StackAlreadyExists error for the caller to decide if they want to ignore the
			// operation or fail the program.
			return &ErrStackAlreadyExists{
				stackName: envStackName(env),
				parentErr: err,
			}
		}
		return err
	}
	return nil
}

// WaitForEnvironmentCreation will block until the environment's CloudFormation stack has completed or errored.
func (cf CloudFormation) WaitForEnvironmentCreation(env *archer.Environment) error {
	name := envStackName(env)
	if err := cf.client.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
		StackName: &name,
	}); err != nil {
		return fmt.Errorf("failed to create stack %s: %w", name, err)
	}
	return nil
}

func (cf CloudFormation) deployChangeSet(in *cloudformation.CreateChangeSetInput) error {
	set, err := cf.createChangeSet(in)
	if err != nil {
		return err
	}
	if err := cf.waitForChangeSetCreation(set); err != nil {
		return err
	}
	if err := cf.executeChangeSet(set); err != nil {
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
	}, nil
}

func (cf CloudFormation) waitForChangeSetCreation(set *changeSet) error {
	if err := cf.client.WaitUntilChangeSetCreateComplete(&cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(set.name),
		StackName:     aws.String(set.stackID),
	}); err != nil {
		return fmt.Errorf("failed to wait for changeSet creation %s: %w", set, err)
	}
	return nil
}

func (cf CloudFormation) executeChangeSet(set *changeSet) error {
	if _, err := cf.client.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(set.name),
		StackName:     aws.String(set.stackID),
	}); err != nil {
		return fmt.Errorf("failed to execute changeSet %s: %w", set, err)
	}
	return nil
}

func (cf CloudFormation) describeChangeSet(set *changeSet) ([]*cloudformation.Change, error) {
	var changes []*cloudformation.Change
	var nextToken *string
	for {
		out, err := cf.client.DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
			ChangeSetName: aws.String(set.name),
			StackName:     aws.String(set.stackID),
			NextToken:     nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe changeSet %s: %w", set, err)
		}
		changes = append(changes, out.Changes...)
		nextToken = out.NextToken

		if nextToken == nil { // no more results left
			break
		}
	}
	return changes, nil
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

func envStackName(env *archer.Environment) string {
	return fmt.Sprintf("%s-%s", env.Project, env.Name)
}
