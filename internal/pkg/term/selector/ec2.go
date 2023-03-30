// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
)

// VPCSubnetLister list VPCs and subnets.
type VPCSubnetLister interface {
	ListVPCs() ([]ec2.VPC, error)
	ListVPCSubnets(vpcID string) (*ec2.VPCSubnets, error)
}

// EC2Select is a selector for Ec2 resources.
type EC2Select struct {
	prompt Prompter
	ec2Svc VPCSubnetLister
}

// NewEC2Select returns a new selector that chooses Ec2 resources.
func NewEC2Select(prompt Prompter, ec2Client VPCSubnetLister) *EC2Select {
	return &EC2Select{
		prompt: prompt,
		ec2Svc: ec2Client,
	}
}

// VPC has the user select an available VPC.
func (s *EC2Select) VPC(msg, help string) (string, error) {
	vpcs, err := s.ec2Svc.ListVPCs()
	if err != nil {
		return "", fmt.Errorf("list VPC ID: %w", err)
	}
	if len(vpcs) == 0 {
		return "", ErrVPCNotFound
	}
	var options []string
	for _, vpc := range vpcs {
		stringifiedVPC := vpc.String()
		options = append(options, stringifiedVPC)
	}
	vpc, err := s.prompt.SelectOne(
		msg, help,
		options,
		prompt.WithFinalMessage("VPC:"))
	if err != nil {
		return "", fmt.Errorf("select VPC: %w", err)
	}
	extractedVPC, err := ec2.ExtractVPC(vpc)
	if err != nil {
		return "", fmt.Errorf("extract VPC ID: %w", err)
	}
	return extractedVPC.ID, nil
}

// SubnetsInput holds the arguments for the subnet selector.
type SubnetsInput struct {
	Msg   string
	Help  string
	VPCID string

	IsPublic bool
}

// Subnets has the user multiselect subnets given the VPC ID.
func (s *EC2Select) Subnets(in SubnetsInput) ([]string, error) {
	return s.selectFromVPCSubnets(in)
}

func (s *EC2Select) selectFromVPCSubnets(in SubnetsInput) ([]string, error) {
	allSubnets, err := s.ec2Svc.ListVPCSubnets(in.VPCID)
	if err != nil {
		return nil, fmt.Errorf("list subnets for VPC %s: %w", in.VPCID, err)
	}
	if in.IsPublic {
		return s.selectSubnets(in.Msg, in.Help, allSubnets.Public, prompt.WithFinalMessage("Public subnets:"))
	}
	return s.selectSubnets(in.Msg, in.Help, allSubnets.Private, prompt.WithFinalMessage("Private subnets:"))
}

func (s *EC2Select) selectSubnets(msg, help string, subnets []ec2.Subnet, opts ...prompt.PromptConfig) ([]string, error) {
	if len(subnets) == 0 {
		return nil, ErrSubnetsNotFound
	}
	var options []string
	for _, subnet := range subnets {
		stringifiedSubnet := subnet.String()
		options = append(options, stringifiedSubnet)
	}
	selectedSubnets, err := s.prompt.MultiSelect(
		msg, help,
		options,
		nil,
		opts...)
	if err != nil {
		return nil, err
	}
	var extractedSubnets []string
	for _, s := range selectedSubnets {
		extractedSubnet, err := ec2.ExtractSubnet(s)
		if err != nil {
			return nil, fmt.Errorf("extract subnet ID: %w", err)
		}
		extractedSubnets = append(extractedSubnets, extractedSubnet.ID)
	}
	return extractedSubnets, nil
}
