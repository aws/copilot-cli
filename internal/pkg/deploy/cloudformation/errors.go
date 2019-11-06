// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
)

// ErrStackAlreadyExists occurs when a CloudFormation stack already exists with a given name.
type ErrStackAlreadyExists struct {
	stackName string
	parentErr error
}

func (err *ErrStackAlreadyExists) Error() string {
	return fmt.Sprintf("stack %s already exists", err.stackName)
}

// Unwrap returns the original CloudFormation error.
func (err *ErrStackAlreadyExists) Unwrap() error {
	return err.parentErr
}

// ErrNotExecutableChangeSet occurs when the change set cannot be executed.
type ErrNotExecutableChangeSet struct {
	set *changeSet
}

func (err *ErrNotExecutableChangeSet) Error() string {
	return fmt.Sprintf("cannot execute change set %s because status is %s with reason %s", err.set, err.set.executionStatus, err.set.statusReason)
}

// ErrTemplateNotFound occurs when we can't find a predefined template.
type ErrTemplateNotFound struct {
	templateLocation string
	parentErr        error
}

func (err *ErrTemplateNotFound) Error() string {
	return fmt.Sprintf("failed to find the cloudformation template at %s", err.templateLocation)
}

// Is returns true if the target's template location and parent error are equal to this error's template location and parent error.
func (err *ErrTemplateNotFound) Is(target error) bool {
	t, ok := target.(*ErrTemplateNotFound)
	if !ok {
		return false
	}
	return (err.templateLocation == t.templateLocation) &&
		(errors.Is(err.parentErr, t.parentErr))
}

// Unwrap returns the original error.
func (err *ErrTemplateNotFound) Unwrap() error {
	return err.parentErr
}

// ErrStackSetOutOfDate occurs when we try to read and then update a StackSet but
// between reading it and actually updating it, someone else either started or completed
// an update.
type ErrStackSetOutOfDate struct {
	projectName string
	parentErr   error
}

func (err *ErrStackSetOutOfDate) Error() string {
	return fmt.Sprintf("cannot update project resources for project %s because the stack set update was out of date (feel free to try again)", err.projectName)
}

// Is returns true if the target's template location and parent error are equal to this error's template location and parent error.
func (err *ErrStackSetOutOfDate) Is(target error) bool {
	t, ok := target.(*ErrStackSetOutOfDate)
	if !ok {
		return false
	}
	return err.projectName == t.projectName
}
