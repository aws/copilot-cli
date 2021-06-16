// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ec2 provides a client to make API requests to Amazon Elastic Compute Cloud.
package ec2

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	defaultForAZFilterName  = "default-for-az"
	internetGatewayIDPrefix = "igw-"

	// TagFilterName is the filter name format for tag filters
	TagFilterName = "tag:%s"
)

var (
	// FilterForDefaultVPCSubnets is a pre-defined filter for the default subnets at the availability zone.
	FilterForDefaultVPCSubnets = Filter{
		Name:   defaultForAZFilterName,
		Values: []string{"true"},
	}
)

type api interface {
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(*ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeVpcAttribute(input *ec2.DescribeVpcAttributeInput) (*ec2.DescribeVpcAttributeOutput, error)
	DescribeNetworkInterfaces(input *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error)
	DescribeRouteTables(input *ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error)
}

// Filter contains the name and values of a filter.
type Filter struct {
	// Name of a filter that will be applied to subnets,
	// for available filter names see: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html.
	Name string
	// Value of the filter.
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

// VPCResource contains the ID and name of a VPCResource.
type VPCResource struct {
	ID   string
	Name string
}

// String formats the elements of a VPC into a display-ready string.
// For example: VPCResource{"ID": "vpc-0576efeea396efee2", "Name": "video-store-test"}
// will return "vpc-0576efeea396efee2 (copilot-video-store-test)".
// while VPCResource{"ID": "subnet-018ccb78d353cec9b", "Name": "public-subnet-1"}
// will return "subnet-018ccb78d353cec9b (public-subnet-1)"
func (v *VPCResource) String() string {
	if v.Name != "" {
		return fmt.Sprintf("%s (%s)", v.ID, v.Name)
	}
	return v.ID
}

// ExtractVPCResource extracts the VPC ID from the VPC display string.
// For example: vpc-0576efeea396efee2 (copilot-video-store-test)
// will return VPC{ID: "vpc-0576efeea396efee2", Name: "copilot-video-store-test"}.
func ExtractVPCResource(label string) (*VPCResource, error) {
	if label == "" {
		return nil, fmt.Errorf("extract VPC resource ID from string: %s", label)
	}
	splitVPCResource := strings.SplitN(label, " ", 2)
	// TODO: switch to regex to make more robust
	var name string
	if len(splitVPCResource) == 2 {
		name = strings.Trim(splitVPCResource[1], "()")
	}
	return &VPCResource{
		ID:   splitVPCResource[0],
		Name: name,
	}, nil
}

// PublicIP returns the public ip associated with the network interface.
func (c *EC2) PublicIP(eni string) (string, error) {
	response, err := c.client.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: aws.StringSlice([]string{eni}),
	})
	if err != nil {
		return "", fmt.Errorf("describe network interface with ENI %s: %w", eni, err)
	}

	// `response.NetworkInterfaces` contains at least one result; if no matching ENI is found, the API call will return
	// an error instead of an empty list of `NetworkInterfaces` (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNetworkInterfaces.html)
	association := response.NetworkInterfaces[0].Association
	if association == nil {
		return "", fmt.Errorf("no association information found for ENI %s", eni)
	}

	return aws.StringValue(association.PublicIp), nil
}

// ListVPCs returns names and IDs (or just IDs, if Name tag does not exist) of all VPCs.
func (c *EC2) ListVPCs() ([]VPCResource, error) {
	var ec2vpcs []*ec2.Vpc
	response, err := c.client.DescribeVpcs(&ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("describe VPCs: %w", err)
	}
	ec2vpcs = append(ec2vpcs, response.Vpcs...)

	for response.NextToken != nil {
		response, err = c.client.DescribeVpcs(&ec2.DescribeVpcsInput{
			NextToken: response.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe VPCs: %w", err)
		}
		ec2vpcs = append(ec2vpcs, response.Vpcs...)
	}
	var vpcs []VPCResource
	for _, vpc := range ec2vpcs {
		var name string
		for _, tag := range vpc.Tags {
			if aws.StringValue(tag.Key) == "Name" {
				name = aws.StringValue(tag.Value)
			}
		}
		vpcs = append(vpcs, VPCResource{
			ID:   aws.StringValue(vpc.VpcId),
			Name: name,
		})
	}
	return vpcs, nil
}

// HasDNSSupport returns if DNS resolution is enabled for the VPC.
func (c *EC2) HasDNSSupport(vpcID string) (bool, error) {
	resp, err := c.client.DescribeVpcAttribute(&ec2.DescribeVpcAttributeInput{
		VpcId:     aws.String(vpcID),
		Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport),
	})
	if err != nil {
		return false, fmt.Errorf("describe %s attribute for VPC %s: %w", ec2.VpcAttributeNameEnableDnsSupport, vpcID, err)
	}
	return aws.BoolValue(resp.EnableDnsSupport.Value), nil
}

// VPCSubnets are all subnets within a VPC.
type VPCSubnets struct {
	Public  []VPCResource
	Private []VPCResource
}

// ListVPCSubnets lists all subnets given a VPC ID. Note that public subnets
// are subnets associated with a route table that routed by an internet gateway.
// And the rest subnets are private subnets.
func (c *EC2) ListVPCSubnets(vpcID string) (*VPCSubnets, error) {
	vpcFilter := Filter{
		Name:   "vpc-id",
		Values: []string{vpcID},
	}
	respRouteTables, err := c.routeTables(vpcFilter)
	if err != nil {
		return nil, err
	}
	publicSubnetMap := make(map[string]bool)
	for _, routeTable := range respRouteTables {
		var igwAttached bool
		for _, route := range routeTable.Routes {
			if strings.HasPrefix(aws.StringValue(route.GatewayId), internetGatewayIDPrefix) {
				igwAttached = true
				break
			}
		}
		if igwAttached {
			for _, association := range routeTable.Associations {
				publicSubnetMap[aws.StringValue(association.SubnetId)] = true
			}
		}
	}
	var publicSubnets, privateSubnets []VPCResource
	respSubnets, err := c.subnets(vpcFilter)
	if err != nil {
		return nil, err
	}
	for _, subnet := range respSubnets {
		var name string
		for _, tag := range subnet.Tags {
			if aws.StringValue(tag.Key) == "Name" {
				name = aws.StringValue(tag.Value)
			}
		}
		s := VPCResource{
			ID:   aws.StringValue(subnet.SubnetId),
			Name: name,
		}
		if _, ok := publicSubnetMap[s.ID]; ok {
			publicSubnets = append(publicSubnets, s)
		} else {
			privateSubnets = append(privateSubnets, s)
		}
	}
	return &VPCSubnets{
		Public:  publicSubnets,
		Private: privateSubnets,
	}, nil
}

// SubnetIDs finds the subnet IDs with optional filters.
func (c *EC2) SubnetIDs(filters ...Filter) ([]string, error) {
	subnets, err := c.subnets(filters...)
	if err != nil {
		return nil, err
	}

	subnetIDs := make([]string, len(subnets))
	for idx, subnet := range subnets {
		subnetIDs[idx] = aws.StringValue(subnet.SubnetId)
	}
	return subnetIDs, nil
}

// SecurityGroups finds the security group IDs with optional filters.
func (c *EC2) SecurityGroups(filters ...Filter) ([]string, error) {
	inputFilters := toEC2Filter(filters)

	response, err := c.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: inputFilters,
	})

	if err != nil {
		return nil, fmt.Errorf("describe security groups: %w", err)
	}

	securityGroups := make([]string, len(response.SecurityGroups))
	for idx, sg := range response.SecurityGroups {
		securityGroups[idx] = aws.StringValue(sg.GroupId)
	}
	return securityGroups, nil
}

func (c *EC2) subnets(filters ...Filter) ([]*ec2.Subnet, error) {
	inputFilters := toEC2Filter(filters)
	var subnets []*ec2.Subnet
	response, err := c.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: inputFilters,
	})
	if err != nil {
		return nil, fmt.Errorf("describe subnets: %w", err)
	}
	subnets = append(subnets, response.Subnets...)

	for response.NextToken != nil {
		response, err = c.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters:   inputFilters,
			NextToken: response.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe subnets: %w", err)
		}
		subnets = append(subnets, response.Subnets...)
	}

	return subnets, nil
}

func (c *EC2) routeTables(filters ...Filter) ([]*ec2.RouteTable, error) {
	inputFilters := toEC2Filter(filters)
	var routeTables []*ec2.RouteTable
	input := &ec2.DescribeRouteTablesInput{
		Filters: inputFilters,
	}
	for {
		resp, err := c.client.DescribeRouteTables(input)
		if err != nil {
			return nil, fmt.Errorf("describe route tables: %w", err)
		}
		routeTables = append(routeTables, resp.RouteTables...)
		if resp.NextToken == nil {
			break
		}
		input.NextToken = resp.NextToken
	}
	return routeTables, nil
}

func toEC2Filter(filters []Filter) []*ec2.Filter {
	var ec2Filter []*ec2.Filter
	for _, filter := range filters {
		ec2Filter = append(ec2Filter, &ec2.Filter{
			Name:   aws.String(filter.Name),
			Values: aws.StringSlice(filter.Values),
		})
	}
	return ec2Filter
}
