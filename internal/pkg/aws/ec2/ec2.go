// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ec2 provides a client to make API requests to Amazon Elastic Compute Cloud.
package ec2

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)


const (
	defaultForAZFilterName = "default-for-az"
)

// Names for tag filters
var (
	TagFilterNameForApp = fmt.Sprintf("tag:%s", deploy.AppTagKey)
	TagFilterNameForEnv = fmt.Sprintf("tag:%s", deploy.EnvTagKey)
)

var (
	// FilterForDefaultVPCSubnets is a pre-defined filter for the default subnets at the availability zone.
	FilterForDefaultVPCSubnets = Filter {
		Name:   defaultForAZFilterName,
		Values: []string{"true"},
	}
)

type api interface {
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(*ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
}

// Filter contains the name and values of a filter.
type Filter struct {
	// Name is the name of a filter that will be applied to subnets, for available filter names see: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html
	Name string
	// Value is the value of the filter
	Values []string
}

// EC2 wraps an AWS EC2 client.
type EC2 struct {
	client api
}

// New returns a EC2 configured against the input session.
func New(s *session.Session) *EC2 {
	return &EC2{
		client: ec2.New(s),
	}
}

// GetSubnetIDs finds the subnet IDs with optional filters
func (c *EC2) GetSubnetIDs(filters ...Filter) ([]string, error) {
	inputFilters := toEC2Filter(filters)

	response, err := c.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: inputFilters,
	})

	if err != nil {
		return nil, fmt.Errorf("get subnets: %w", err)
	}

	subnetIDs := make([]string, len(response.Subnets))
	for idx, subnet := range response.Subnets {
		subnetIDs[idx] = aws.StringValue(subnet.SubnetId)
	}
	return subnetIDs, nil
}

// GetSecurityGroups finds the security group IDs with optional filters
func (c *EC2) GetSecurityGroups(filters ...Filter) ([]string, error) {
	inputFilters := toEC2Filter(filters)

	response, err := c.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: inputFilters,
	})

	if err != nil {
		return nil, fmt.Errorf("get security groups: %w", err)
	}

	securityGroups := make([]string, len(response.SecurityGroups))
	for idx, sg := range response.SecurityGroups {
		securityGroups[idx] = aws.StringValue(sg.GroupId)
	}
	return securityGroups, nil
}

func toEC2Filter(filters []Filter) []*ec2.Filter {
	var ec2Filter []*ec2.Filter
	for _, filter := range filters {
		ec2Filter = append(ec2Filter, &ec2.Filter{
			Name: aws.String(filter.Name),
			Values: aws.StringSlice(filter.Values),
		})
	}
	return ec2Filter
}
