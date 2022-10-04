// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package iam provides a client to make API requests to the AWS Identity and Access Management service.
package iam

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

const (
	ecsServiceName = "ecs.amazonaws.com"
)

type api interface {
	ListRoleTags(input *iam.ListRoleTagsInput) (*iam.ListRoleTagsOutput, error)
	DeleteRolePolicy(input *iam.DeleteRolePolicyInput) (*iam.DeleteRolePolicyOutput, error)
	ListRolePolicies(input *iam.ListRolePoliciesInput) (*iam.ListRolePoliciesOutput, error)
	DeleteRole(input *iam.DeleteRoleInput) (*iam.DeleteRoleOutput, error)
	CreateServiceLinkedRole(input *iam.CreateServiceLinkedRoleInput) (*iam.CreateServiceLinkedRoleOutput, error)
	ListPolicies(input *iam.ListPoliciesInput) (*iam.ListPoliciesOutput, error)
}

// IAM wraps the AWS SDK's IAM client.
type IAM struct {
	client api
}

// New returns an IAM client configured against the input session.
func New(s *session.Session) *IAM {
	return &IAM{
		client: iam.New(s),
	}
}

// ListRoleTags gathers all the tags associated with an IAM role.
func (c *IAM) ListRoleTags(roleName string) (map[string]string, error) {
	tags := make(map[string]string)
	var marker *string
	for {
		out, err := c.client.ListRoleTags(&iam.ListRoleTagsInput{
			RoleName: aws.String(roleName),
			Marker:   marker,
		})
		if err != nil {
			return nil, fmt.Errorf("list role tags for role %s and marker %v: %w", roleName, marker, err)
		}
		for _, tag := range out.Tags {
			tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
		}
		if !aws.BoolValue(out.IsTruncated) {
			return tags, nil
		}
		marker = out.Marker
	}
}

// DeleteRole deletes an IAM role based on its ARN.
// If the role does not exist it returns nil.
func (c *IAM) DeleteRole(roleNameOrARN string) error {
	roleName := roleNameOrARN
	if parsed, err := arn.Parse(roleNameOrARN); err == nil {
		// The parameter is an ARN instead!
		// Sample ARN format: arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole
		roleName = strings.TrimPrefix(parsed.Resource, "role/")
	}

	if err := c.deleteRolePolicies(roleName); err != nil {
		return err
	}
	if _, err := c.client.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	}); err != nil {
		if isNotExistErr(err) {
			// The role does not exist, exit successfully.
			return nil
		}
		return fmt.Errorf("delete role named %s: %w", roleName, err)
	}
	return nil
}

// CreateECSServiceLinkedRole creates a Service-Linked Role for Amazon ECS.
// This role is necessary so that Amazon ECS can call AWS APIs.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using-service-linked-roles.html
func (c *IAM) CreateECSServiceLinkedRole() error {
	if _, err := c.client.CreateServiceLinkedRole(&iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String(ecsServiceName),
	}); err != nil {
		return fmt.Errorf("create service linked role for %s: %w", ecsServiceName, err)
	}
	return nil
}

// ListPolicyNames returns a list of local policy names.
func (c *IAM) ListPolicyNames() ([]string, error) {
	var policies []*iam.Policy
	var marker *string
	for {
		output, err := c.client.ListPolicies(&iam.ListPoliciesInput{
			Marker:            marker,
			Scope:             aws.String("Local"),
			PolicyUsageFilter: aws.String("PermissionsBoundary"),
		})
		if err != nil {
			return nil, fmt.Errorf("list IAM policies: %w", err)
		}
		policies = append(policies, output.Policies...)
		if !aws.BoolValue(output.IsTruncated) {
			break
		}
		marker = output.Marker
	}
	var policyNames = make([]string, len(policies))
	for i, policy := range policies {
		policyNames[i] = aws.StringValue(policy.PolicyName)
	}
	return policyNames, nil
}

func (c *IAM) deleteRolePolicies(roleName string) error {
	policyNames, err := c.listRolePolicyNames(roleName)
	if err != nil {
		return err
	}
	for _, policyName := range policyNames {
		if _, err := c.client.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
			PolicyName: policyName,
			RoleName:   aws.String(roleName),
		}); err != nil {
			return fmt.Errorf("delete policy named %s in role %s: %w", aws.StringValue(policyName), roleName, err)
		}
	}
	return nil
}

func (c *IAM) listRolePolicyNames(roleName string) ([]*string, error) {
	var policyNames []*string
	var marker *string
	for {
		out, err := c.client.ListRolePolicies(&iam.ListRolePoliciesInput{
			Marker:   marker,
			RoleName: aws.String(roleName),
		})
		if err != nil {
			if isNotExistErr(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("list role policies for role %s: %v", roleName, err)
		}
		policyNames = append(policyNames, out.PolicyNames...)
		if !aws.BoolValue(out.IsTruncated) {
			return policyNames, nil
		}
		marker = out.Marker
	}
}

func isNotExistErr(err error) bool {
	aerr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	switch aerr.Code() {
	case iam.ErrCodeNoSuchEntityException:
		return true
	default:
		return false
	}
}
