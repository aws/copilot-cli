// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ec2 provides a client to make API requests to Amazon Elastic Compute Cloud.
package ec2

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

var (
	fmtFilterTagApp = fmt.Sprintf("tag:%s", deploy.AppTagKey)
	fmtFilterTagEnv = fmt.Sprintf("tag:%s", deploy.EnvTagKey)
	defaultFilter   = ec2.Filter{
		Name:   aws.String(filterDefault),
		Values: aws.StringSlice([]string{"true"}),
	}
)

const (
	filterDefault = "default-for-az"
)

type api interface {
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(*ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
}

type EC2 struct {
	client api
}

func New(s *session.Session) *EC2 {
	return &EC2{
		client: ec2.New(s),
	}
}

// GetSubnetIDs finds the subnet IDs associated with the environment of the application; if env is a None env,
// it finds the default subnet IDs.
func (c *EC2) GetSubnetIDs(app string, env string) ([]string, error) {
	if env == config.EnvNameNone {
		return c.getDefaultSubnetIDs()
	}

	filters := []*ec2.Filter{
		{
			Name:   aws.String(fmtFilterTagApp),
			Values: aws.StringSlice([]string{app}),
		},
		{
			Name:   aws.String(fmtFilterTagEnv),
			Values: aws.StringSlice([]string{env}),
		},
	}
	return c.getSubnetIDs(filters)
}

// GetSecurityGroups finds the security group IDs associated with the environment of the application;
// if env is a None env, it returns nil
func (c *EC2) GetSecurityGroups(app string, env string) ([]string, error) {
	if env == config.EnvNameNone {
		return nil, nil
	}

	filters := []*ec2.Filter{
		{
			Name:   aws.String(fmtFilterTagApp),
			Values: aws.StringSlice([]string{app}),
		},
		{
			Name:   aws.String(fmtFilterTagEnv),
			Values: aws.StringSlice([]string{env}),
		},
	}

	return c.getSecurityGroups(filters)
}

func (c *EC2) getDefaultSubnetIDs() ([]string, error) {
	return c.getSubnetIDs([]*ec2.Filter{&defaultFilter})
}

func (c *EC2) getSubnetIDs(filters []*ec2.Filter) ([]string, error) {
	response, err := c.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("find subnets: %w", err)
	}

	if len(response.Subnets) == 0 {
		return nil, errors.New("no subnets found")
	}

	subnetIDs := make([]string, len(response.Subnets))
	for idx, subnet := range response.Subnets {
		subnetIDs[idx] = aws.StringValue(subnet.SubnetId)
	}
	return subnetIDs, nil
}

func (c *EC2) getSecurityGroups(filters []*ec2.Filter) ([]string, error) {
	response, err := c.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("get security groups from environment: %w", err)
	}

	securityGroups := make([]string, len(response.SecurityGroups))
	for idx, sg := range response.SecurityGroups {
		securityGroups[idx] = aws.StringValue(sg.GroupId)
	}
	return securityGroups, nil
}
