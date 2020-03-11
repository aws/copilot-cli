// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Stack represents a AWS CloudFormation stack.
type Stack struct {
	name string
	*stackConfig
}

type stackConfig struct {
	template   string
	parameters []*cloudformation.Parameter
	tags       []*cloudformation.Tag
	roleARN    *string
}

// StackOption allows you to initialize a Stack with additional properties.
type StackOption func(s *Stack)

// NewStack creates a stack with the given name and template body.
func NewStack(name, template string, opts ...StackOption) *Stack {
	s := &Stack{
		name: name,
		stackConfig: &stackConfig{
			template: template,
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithParameters passes parameters to a stack.
func WithParameters(params map[string]string) StackOption {
	return func(s *Stack) {
		var flatParams []*cloudformation.Parameter
		for k, v := range params {
			flatParams = append(flatParams, &cloudformation.Parameter{
				ParameterKey:   aws.String(k),
				ParameterValue: aws.String(v),
			})
		}
		s.parameters = flatParams
	}
}

// WithTags applies the tags to a stack.
func WithTags(tags map[string]string) StackOption {
	return func(s *Stack) {
		var flatTags []*cloudformation.Tag
		for k, v := range tags {
			flatTags = append(flatTags, &cloudformation.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		s.tags = flatTags
	}
}

// WithRoleARN specifies the role that CloudFormation will assume when creating the stack.
func WithRoleARN(roleARN string) StackOption {
	return func(s *Stack) {
		s.roleARN = aws.String(roleARN)
	}
}
