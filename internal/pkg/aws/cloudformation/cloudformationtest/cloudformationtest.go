// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformationtest

import (
	"context"

	sdk "github.com/aws/aws-sdk-go/service/cloudformation"
	cfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
)

// Double is a test double for cloudformation.CloudFormation
type Double struct {
	CreateFn                    func(stack *cfn.Stack) (string, error)
	CreateAndWaitFn             func(stack *cfn.Stack) error
	DescribeChangeSetFn         func(changeSetID, stackName string) (*cfn.ChangeSetDescription, error)
	WaitForCreateFn             func(ctx context.Context, stackName string) error
	UpdateFn                    func(stack *cfn.Stack) (string, error)
	UpdateAndWaitFn             func(stack *cfn.Stack) error
	WaitForUpdateFn             func(ctx context.Context, stackName string) error
	DeleteFn                    func(stackName string) error
	DeleteAndWaitFn             func(stackName string) error
	DeleteAndWaitWithRoleARNFn  func(stackName, roleARN string) error
	DescribeFn                  func(name string) (*cfn.StackDescription, error)
	ExistsFn                    func(name string) (bool, error)
	MetadataFn                  func(opt cfn.MetadataOpts) (string, error)
	TemplateBodyFn              func(name string) (string, error)
	TemplateBodyFromChangeSetFn func(changeSetID, stackName string) (string, error)
	OutputsFn                   func(stack *cfn.Stack) (map[string]string, error)
	EventsFn                    func(stackName string) ([]cfn.StackEvent, error)
	StackResourcesFn            func(name string) ([]*cfn.StackResource, error)
	ErrorEventsFn               func(stackName string) ([]cfn.StackEvent, error)
	ListStacksWithTagsFn        func(tags map[string]string) ([]cfn.StackDescription, error)
	DescribeStackEventsFn       func(input *sdk.DescribeStackEventsInput) (*sdk.DescribeStackEventsOutput, error)
	CancelUpdateStackFn         func(stackName string) error
}

// Create calls the stubbed function.
func (d *Double) Create(stack *cfn.Stack) (string, error) {
	return d.CreateFn(stack)
}

// CreateAndWait calls the stubbed function.
func (d *Double) CreateAndWait(stack *cfn.Stack) error {
	return d.CreateAndWaitFn(stack)
}

// DescribeChangeSet calls the stubbed function.
func (d *Double) DescribeChangeSet(id, stack string) (*cfn.ChangeSetDescription, error) {
	return d.DescribeChangeSetFn(id, stack)
}

// WaitForCreate calls the stubbed function.
func (d *Double) WaitForCreate(ctx context.Context, stack string) error {
	return d.WaitForCreateFn(ctx, stack)
}

// Update calls the stubbed function.
func (d *Double) Update(stack *cfn.Stack) (string, error) {
	return d.UpdateFn(stack)
}

// UpdateAndWait calls the stubbed function.
func (d *Double) UpdateAndWait(stack *cfn.Stack) error {
	return d.UpdateAndWaitFn(stack)
}

// WaitForUpdate calls the stubbed function.
func (d *Double) WaitForUpdate(ctx context.Context, stackName string) error {
	return d.WaitForUpdateFn(ctx, stackName)
}

// Delete calls the stubbed function.
func (d *Double) Delete(stackName string) error {
	return d.DeleteFn(stackName)
}

// DeleteAndWait calls the stubbed function.
func (d *Double) DeleteAndWait(stackName string) error {
	return d.DeleteAndWaitFn(stackName)
}

// DeleteAndWaitWithRoleARN calls the stubbed function.
func (d *Double) DeleteAndWaitWithRoleARN(stackName, roleARN string) error {
	return d.DeleteAndWaitWithRoleARNFn(stackName, roleARN)
}

// Describe calls the stubbed function.
func (d *Double) Describe(name string) (*cfn.StackDescription, error) {
	return d.DescribeFn(name)
}

// Exists calls the stubbed function.
func (d *Double) Exists(name string) (bool, error) {
	return d.ExistsFn(name)
}

// Metadata calls the stubbed function.
func (d *Double) Metadata(opt cfn.MetadataOpts) (string, error) {
	return d.MetadataFn(opt)
}

// TemplateBody calls the stubbed function.
func (d *Double) TemplateBody(name string) (string, error) {
	return d.TemplateBodyFn(name)
}

// TemplateBodyFromChangeSet calls the stubbed function.
func (d *Double) TemplateBodyFromChangeSet(changeSetID, stackName string) (string, error) {
	return d.TemplateBodyFromChangeSetFn(changeSetID, stackName)
}

// Outputs calls the stubbed function.
func (d *Double) Outputs(stack *cfn.Stack) (map[string]string, error) {
	return d.OutputsFn(stack)
}

// Events calls the stubbed function.
func (d *Double) Events(stackName string) ([]cfn.StackEvent, error) {
	return d.EventsFn(stackName)
}

// StackResources calls the stubbed function.
func (d *Double) StackResources(name string) ([]*cfn.StackResource, error) {
	return d.StackResourcesFn(name)
}

// ErrorEvents calls the stubbed function.
func (d *Double) ErrorEvents(stackName string) ([]cfn.StackEvent, error) {
	return d.ErrorEventsFn(stackName)
}

// ListStacksWithTags calls the stubbed function.
func (d *Double) ListStacksWithTags(tags map[string]string) ([]cfn.StackDescription, error) {
	return d.ListStacksWithTagsFn(tags)
}

// DescribeStackEvents calls the stubbed function.
func (d *Double) DescribeStackEvents(input *sdk.DescribeStackEventsInput) (*sdk.DescribeStackEventsOutput, error) {
	return d.DescribeStackEventsFn(input)
}

// CancelUpdateStack calls the stubbed function.
func (d *Double) CancelUpdateStack(stackName string) error {
	return d.CancelUpdateStackFn(stackName)
}
