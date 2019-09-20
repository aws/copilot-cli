// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
package cloudformation

import (
	"fmt"
	"strconv"

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

// DeployEnvironment creates an environment CloudFormation stack
func (cf CloudFormation) DeployEnvironment(env archer.Environment) error {
	template, err := cf.box.FindString(environmentTemplate)

	if err != nil {
		return err
	}

	stackName := stackName(env)

	in := &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: &template,
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String(includeLoadBalancerParamKey),
				ParameterValue: aws.String(strconv.FormatBool(env.PublicLoadBalancer)),
			},
		},
	}

	_, err = cf.client.CreateStack(in)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudformation.ErrCodeAlreadyExistsException:
				return nil
			}
		}
		return fmt.Errorf("failed to deploy the environment %s with CloudFormation due to: %w", env.Name, err)
	}

	return nil
}

// Wait will block until CloudFormation stack has completed or errored.
func (cf CloudFormation) Wait(env archer.Environment) error {
	stackName := stackName(env)

	in := &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	}

	return cf.client.WaitUntilStackCreateComplete(in)
}

func stackName(env archer.Environment) string {
	return fmt.Sprintf("%s-%s", env.Project, env.Name)
}
