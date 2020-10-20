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

type api interface {
	DeleteRolePolicy(input *iam.DeleteRolePolicyInput) (*iam.DeleteRolePolicyOutput, error)
	ListRolePolicies(input *iam.ListRolePoliciesInput) (*iam.ListRolePoliciesOutput, error)
	DeleteRole(input *iam.DeleteRoleInput) (*iam.DeleteRoleOutput, error)
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

// DeleteRole deletes an IAM role based on its ARN.
// If the role does not exist it returns nil.
func (c *IAM) DeleteRole(roleARN string) error {
	parsed, err := arn.Parse(roleARN)
	if err != nil {
		return fmt.Errorf("parse role ARN %s: %w", roleARN, err)
	}

	roleName := strings.TrimPrefix(parsed.Resource, "role/") // Sample ARN format: arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole
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
