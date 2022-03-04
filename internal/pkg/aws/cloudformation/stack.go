// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// Stack represents a AWS CloudFormation stack.
type Stack struct {
	Name string
	*stackConfig
}

type stackConfig struct {
	TemplateBody    string
	TemplateURL     string
	Parameters      []*cloudformation.Parameter
	Tags            []*cloudformation.Tag
	RoleARN         *string
	DisableRollback bool
}

// StackOption allows you to initialize a Stack with additional properties.
type StackOption func(s *Stack)

// NewStack creates a stack with the given name and template body.
func NewStack(name, template string, opts ...StackOption) *Stack {
	s := &Stack{
		Name: name,
		stackConfig: &stackConfig{
			TemplateBody: template,
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewStackWithURL creates a stack with a URL to the template.
func NewStackWithURL(name, templateURL string, opts ...StackOption) *Stack {
	s := &Stack{
		Name: name,
		stackConfig: &stackConfig{
			TemplateURL: templateURL,
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
		s.Parameters = flatParams
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
		s.Tags = flatTags
	}
}

// WithRoleARN specifies the role that CloudFormation will assume when creating the stack.
func WithRoleARN(roleARN string) StackOption {
	return func(s *Stack) {
		s.RoleARN = aws.String(roleARN)
	}
}

// WithDisableRollback disables CloudFormation's automatic stack rollback upon failure for the stack.
func WithDisableRollback() StackOption {
	return func(s *Stack) {
		s.DisableRollback = true
	}
}

// StackEvent is an alias the SDK's StackEvent type.
type StackEvent cloudformation.StackEvent

// StackDescription is an alias the SDK's Stack type.
type StackDescription cloudformation.Stack

// StackResource is an alias the SDK's StackResource type.
type StackResource cloudformation.StackResource

// SDK returns the underlying struct from the AWS SDK.
func (d *StackDescription) SDK() *cloudformation.Stack {
	raw := cloudformation.Stack(*d)
	return &raw
}
