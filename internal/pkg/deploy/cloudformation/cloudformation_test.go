// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
)

const (
	mockTemplate = "mockTemplate"
)

type mockCloudFormation struct {
	cloudformationiface.CloudFormationAPI

	t                             *testing.T
	mockCreateStackSet            func(t *testing.T, in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error)
	mockDescribeStackSet          func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error)
	mockUpdateStackSet            func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error)
	mockDeleteStackSet            func(t *testing.T, in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error)
	mockListStackInstances        func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error)
	mockCreateStackInstances      func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error)
	mockDeleteStackInstances      func(t *testing.T, in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error)
	mockDescribeStackSetOperation func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error)
}

func (cf mockCloudFormation) CreateStackSet(in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error) {
	return cf.mockCreateStackSet(cf.t, in)
}

func (cf mockCloudFormation) DescribeStackSet(in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
	return cf.mockDescribeStackSet(cf.t, in)
}

func (cf mockCloudFormation) UpdateStackSet(in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
	return cf.mockUpdateStackSet(cf.t, in)
}

func (cf mockCloudFormation) DeleteStackSet(in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error) {
	return cf.mockDeleteStackSet(cf.t, in)
}

func (cf mockCloudFormation) ListStackInstances(in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
	return cf.mockListStackInstances(cf.t, in)
}

func (cf mockCloudFormation) CreateStackInstances(in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
	return cf.mockCreateStackInstances(cf.t, in)
}

func (cf mockCloudFormation) DeleteStackInstances(in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error) {
	return cf.mockDeleteStackInstances(cf.t, in)
}

func (cf mockCloudFormation) DescribeStackSetOperation(in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
	return cf.mockDescribeStackSetOperation(cf.t, in)
}

func boxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(stack.EnvTemplatePath, mockTemplate)

	return box
}
