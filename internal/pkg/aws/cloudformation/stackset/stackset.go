// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package stackset provides a client to make API requests to an AWS CloudFormation StackSet resource.
package stackset

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type api interface {
	CreateStackSet(*cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error)
	UpdateStackSet(*cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error)
	ListStackSetOperations(input *cloudformation.ListStackSetOperationsInput) (*cloudformation.ListStackSetOperationsOutput, error)
	DeleteStackSet(*cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error)
	DescribeStackSet(*cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error)
	DescribeStackSetOperation(*cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error)

	CreateStackInstances(*cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error)
	DeleteStackInstances(*cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error)
	ListStackInstances(*cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error)
}

const (
	opStatusSucceeded = "SUCCEEDED"
	opStatusStopped   = "STOPPED"
	opStatusFailed    = "FAILED"
)

// StackSet represents an AWS CloudFormation client to interact with stack sets.
type StackSet struct {
	client api
}

// New creates a new client to make requests against stack sets.
func New(s *session.Session) *StackSet {
	return &StackSet{
		client: cloudformation.New(s),
	}
}

// CreateOrUpdateOption allows to initialize or update a stack set with additional properties.
type CreateOrUpdateOption func(interface{})

// Create creates a new stack set resource, if one already exists then do nothing.
func (ss *StackSet) Create(name, template string, opts ...CreateOrUpdateOption) error {
	in := &cloudformation.CreateStackSetInput{
		StackSetName: aws.String(name),
		TemplateBody: aws.String(template),
	}
	for _, opt := range opts {
		opt(in)
	}
	_, err := ss.client.CreateStackSet(in)
	if err != nil {
		if !isAlreadyExistingStackSet(err) {
			return fmt.Errorf("create stack set %s: %w", name, err)
		}
	}
	return nil
}

// Describe returns a description of a created stack set.
func (ss *StackSet) Describe(name string) (Description, error) {
	resp, err := ss.client.DescribeStackSet(&cloudformation.DescribeStackSetInput{
		StackSetName: aws.String(name),
	})
	if err != nil {
		return Description{}, fmt.Errorf("describe stack set %s: %w", name, err)
	}
	return Description{
		ID:       aws.StringValue(resp.StackSet.StackSetId),
		Name:     aws.StringValue(resp.StackSet.StackSetName),
		Template: aws.StringValue(resp.StackSet.TemplateBody),
	}, nil
}

// Update updates a stack set with a new template.
func (ss *StackSet) Update(name, template string, opts ...CreateOrUpdateOption) error {
	if _, err := ss.update(name, template, opts...); err != nil {
		return err
	}
	return nil
}

// UpdateAndWait updates a stack set with a new template, and waits until the operation completes.
func (ss *StackSet) UpdateAndWait(name, template string, opts ...CreateOrUpdateOption) error {
	id, err := ss.update(name, template, opts...)
	if err != nil {
		return err
	}
	return ss.waitForOperation(name, id)
}

// Delete removes all the stack instances from a stack set and then deletes the stack set.
func (ss *StackSet) Delete(name string) error {
	summaries, err := ss.InstanceSummaries(name)
	if err != nil {
		// If the stack set doesn't exist - just move on.
		if isNotFoundStackSet(errors.Unwrap(err)) {
			return nil
		}
		return err
	}

	// We want to delete all the stack instances, so we create a set of account ids and regions.
	uniqueAccounts := make(map[string]bool)
	uniqueRegions := make(map[string]bool)
	for _, summary := range summaries {
		uniqueAccounts[summary.Account] = true
		uniqueRegions[summary.Region] = true
	}

	var regions []string
	var accounts []string
	for account := range uniqueAccounts {
		accounts = append(accounts, account)
	}
	for region := range uniqueRegions {
		regions = append(regions, region)
	}

	// Delete the stack instances for those accounts and regions.
	if len(summaries) > 0 {
		operation, err := ss.client.DeleteStackInstances(&cloudformation.DeleteStackInstancesInput{
			StackSetName: aws.String(name),
			Accounts:     aws.StringSlice(accounts),
			Regions:      aws.StringSlice(regions),
			RetainStacks: aws.Bool(false),
		})
		if err != nil {
			return fmt.Errorf("delete stack instances in regions %v for accounts %v for stackset %s: %w",
				regions, accounts, name, err)
		}
		if err := ss.waitForOperation(name, aws.StringValue(operation.OperationId)); err != nil {
			return err
		}
	}

	// Delete the stack set now that the stack instances are gone.
	if _, err := ss.client.DeleteStackSet(&cloudformation.DeleteStackSetInput{
		StackSetName: aws.String(name),
	}); err != nil {
		if !isNotFoundStackSet(err) {
			return fmt.Errorf("delete stack set %s: %w", name, err)
		}
		// If the stack set doesn't exist, that's fine, move on.
	}
	return nil
}

// CreateInstances creates new stack instances for a stack set within the regions of the specified AWS accounts.
func (ss *StackSet) CreateInstances(name string, accounts, regions []string) error {
	if _, err := ss.createInstances(name, accounts, regions); err != nil {
		return err
	}
	return nil
}

// CreateInstancesAndWait creates new stack instances in the regions of the specified AWS accounts, and waits until the operation completes.
func (ss *StackSet) CreateInstancesAndWait(name string, accounts, regions []string) error {
	id, err := ss.createInstances(name, accounts, regions)
	if err != nil {
		return err
	}
	return ss.waitForOperation(name, id)
}

// InstanceSummariesOption allows to filter instance summaries to retrieve for the stack set.
type InstanceSummariesOption func(input *cloudformation.ListStackInstancesInput)

// InstanceSummaries returns a list of unique identifiers for all the stack instances in a stack set.
func (ss *StackSet) InstanceSummaries(name string, opts ...InstanceSummariesOption) ([]InstanceSummary, error) {
	in := &cloudformation.ListStackInstancesInput{
		StackSetName: aws.String(name),
	}
	for _, opt := range opts {
		opt(in)
	}
	resp, err := ss.client.ListStackInstances(in)
	if err != nil {
		return nil, fmt.Errorf("list stack instances for stack set %s: %w", name, err)
	}
	var summaries []InstanceSummary
	for _, summary := range resp.Summaries {
		summaries = append(summaries, InstanceSummary{
			StackID: aws.StringValue(summary.StackId),
			Account: aws.StringValue(summary.Account),
			Region:  aws.StringValue(summary.Region),
		})
	}
	return summaries, nil
}

func (ss *StackSet) update(name, template string, opts ...CreateOrUpdateOption) (string, error) {
	in := &cloudformation.UpdateStackSetInput{
		StackSetName: aws.String(name),
		TemplateBody: aws.String(template),
		OperationPreferences: &cloudformation.StackSetOperationPreferences{
			RegionConcurrencyType: aws.String(cloudformation.RegionConcurrencyTypeParallel),
		},
	}
	for _, opt := range opts {
		opt(in)
	}
	resp, err := ss.client.UpdateStackSet(in)
	if err != nil {
		if isOutdatedStackSet(err) {
			return "", &ErrStackSetOutOfDate{
				stackSetName: name,
				parentErr:    err,
			}
		}
		return "", fmt.Errorf("update stack set %s: %w", name, err)
	}
	return aws.StringValue(resp.OperationId), nil
}

func (ss *StackSet) createInstances(name string, accounts, regions []string) (string, error) {
	resp, err := ss.client.CreateStackInstances(&cloudformation.CreateStackInstancesInput{
		StackSetName: aws.String(name),
		Accounts:     aws.StringSlice(accounts),
		Regions:      aws.StringSlice(regions),
	})
	if err != nil {
		return "", fmt.Errorf("create stack instances for stack set %s in regions %v for accounts %v: %w",
			name, regions, accounts, err)
	}
	return aws.StringValue(resp.OperationId), nil
}

// WaitForStackSetLastOperationComplete waits until the stackset's last operation completes.
func (ss *StackSet) WaitForStackSetLastOperationComplete(name string) error {
	for {
		resp, err := ss.client.ListStackSetOperations(&cloudformation.ListStackSetOperationsInput{
			StackSetName: aws.String(name),
		})
		if err != nil {
			return fmt.Errorf("list operations for stack set %s: %w", name, err)
		}
		if len(resp.Summaries) == 0 {
			return nil
		}
		operation := resp.Summaries[0]
		switch aws.StringValue(operation.Status) {
		case cloudformation.StackSetOperationStatusRunning:
		case cloudformation.StackSetOperationStatusStopping:
		case cloudformation.StackSetOperationStatusQueued:
		default:
			return nil
		}
		time.Sleep(3 * time.Second)
	}
}

func (ss *StackSet) waitForOperation(name, operationID string) error {
	for {
		response, err := ss.client.DescribeStackSetOperation(&cloudformation.DescribeStackSetOperationInput{
			StackSetName: aws.String(name),
			OperationId:  aws.String(operationID),
		})
		if err != nil {
			return fmt.Errorf("describe operation %s for stack set %s: %w", operationID, name, err)
		}
		if aws.StringValue(response.StackSetOperation.Status) == opStatusSucceeded {
			return nil
		}
		if aws.StringValue(response.StackSetOperation.Status) == opStatusStopped {
			return fmt.Errorf("operation %s for stack set %s was manually stopped", operationID, name)
		}
		if aws.StringValue(response.StackSetOperation.Status) == opStatusFailed {
			return fmt.Errorf("operation %s for stack set %s failed", operationID, name)
		}
		time.Sleep(3 * time.Second)
	}
}

// WithDescription sets a description for a stack set.
func WithDescription(description string) CreateOrUpdateOption {
	return func(input interface{}) {
		switch v := input.(type) {
		case *cloudformation.CreateStackSetInput:
			{
				v.Description = aws.String(description)
			}
		case *cloudformation.UpdateStackSetInput:
			{
				v.Description = aws.String(description)
			}
		}
	}
}

// WithExecutionRoleName sets an execution role name for a stack set.
func WithExecutionRoleName(roleName string) CreateOrUpdateOption {
	return func(input interface{}) {
		switch v := input.(type) {
		case *cloudformation.CreateStackSetInput:
			{
				v.ExecutionRoleName = aws.String(roleName)
			}
		case *cloudformation.UpdateStackSetInput:
			{
				v.ExecutionRoleName = aws.String(roleName)
			}
		}
	}
}

// WithAdministrationRoleARN sets an administration role arn for a stack set.
func WithAdministrationRoleARN(roleARN string) CreateOrUpdateOption {
	return func(input interface{}) {
		switch v := input.(type) {
		case *cloudformation.CreateStackSetInput:
			{
				v.AdministrationRoleARN = aws.String(roleARN)
			}
		case *cloudformation.UpdateStackSetInput:
			{
				v.AdministrationRoleARN = aws.String(roleARN)
			}
		}
	}
}

// WithTags sets tags to all the resources in a stack set.
func WithTags(tags map[string]string) CreateOrUpdateOption {
	return func(input interface{}) {
		var flatTags []*cloudformation.Tag
		for k, v := range tags {
			flatTags = append(flatTags, &cloudformation.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}

		switch v := input.(type) {
		case *cloudformation.CreateStackSetInput:
			{
				v.Tags = flatTags
			}
		case *cloudformation.UpdateStackSetInput:
			{
				v.Tags = flatTags
			}
		}
	}
}

// WithOperationID sets the operation ID of a stack set operation.
// This functional option can only be used while updating a stack set, otherwise it's a no-op.
func WithOperationID(operationID string) CreateOrUpdateOption {
	return func(input interface{}) {
		switch v := input.(type) {
		case *cloudformation.UpdateStackSetInput:
			{
				v.OperationId = aws.String(operationID)
			}
		}
	}
}

// FilterSummariesByAccountID limits the accountID for the stack instance summaries to retrieve.
func FilterSummariesByAccountID(accountID string) InstanceSummariesOption {
	return func(input *cloudformation.ListStackInstancesInput) {
		input.StackInstanceAccount = aws.String(accountID)
	}
}

// FilterSummariesByRegion limits the region for the stack instance summaries to retrieve.
func FilterSummariesByRegion(region string) InstanceSummariesOption {
	return func(input *cloudformation.ListStackInstancesInput) {
		input.StackInstanceRegion = aws.String(region)
	}
}
