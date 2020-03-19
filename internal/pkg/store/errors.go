// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import "fmt"

// ErrNoSuchProject means a project couldn't be found within a specific account and region.
type ErrNoSuchProject struct {
	ProjectName string
	AccountID   string
	Region      string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchProject) Is(target error) bool {
	t, ok := target.(*ErrNoSuchProject)
	if !ok {
		return false
	}
	return e.ProjectName == t.ProjectName &&
		e.AccountID == t.AccountID &&
		e.Region == t.Region
}

func (e *ErrNoSuchProject) Error() string {
	return fmt.Sprintf("couldn't find a project named %s in account %s and region %s",
		e.ProjectName, e.AccountID, e.Region)
}

// ErrProjectAlreadyExists means a project already exists and can't be created again.
type ErrProjectAlreadyExists struct {
	ProjectName string
}

// Is returns whether the provided error equals this error.
func (e *ErrProjectAlreadyExists) Is(target error) bool {
	t, ok := target.(*ErrProjectAlreadyExists)
	if !ok {
		return false
	}
	return e.ProjectName == t.ProjectName
}

func (e *ErrProjectAlreadyExists) Error() string {
	return fmt.Sprintf("a project named %s already exists",
		e.ProjectName)
}

// ErrNoSuchEnvironment means a specific environment couldn't be found in a specific project.
type ErrNoSuchEnvironment struct {
	ProjectName     string
	EnvironmentName string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchEnvironment) Is(target error) bool {
	t, ok := target.(*ErrNoSuchEnvironment)
	if !ok {
		return false
	}
	return e.ProjectName == t.ProjectName &&
		e.EnvironmentName == t.EnvironmentName
}

func (e *ErrNoSuchEnvironment) Error() string {
	return fmt.Sprintf("couldn't find environment %s in the project %s",
		e.EnvironmentName, e.ProjectName)
}

// ErrNoSuchApplication means a specific application couldn't be found in a specific project.
type ErrNoSuchApplication struct {
	ProjectName     string
	ApplicationName string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchApplication) Is(target error) bool {
	t, ok := target.(*ErrNoSuchApplication)
	if !ok {
		return false
	}
	return e.ProjectName == t.ProjectName &&
		e.ApplicationName == t.ApplicationName
}

func (e *ErrNoSuchApplication) Error() string {
	return fmt.Sprintf("couldn't find application %s in the project %s",
		e.ApplicationName, e.ProjectName)
}
