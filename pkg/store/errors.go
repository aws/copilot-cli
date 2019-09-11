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

func (e *ErrNoSuchProject) Error() string {
	return fmt.Sprintf("couldn't find a project named %s in account %s and region %s",
		e.ProjectName, e.AccountID, e.Region)
}

// ErrProjectAlreadyExists means a project already exists and can't be created again.
type ErrProjectAlreadyExists struct {
	ProjectName string
}

func (e *ErrProjectAlreadyExists) Error() string {
	return fmt.Sprintf("a project named %s already exists",
		e.ProjectName)
}

// ErrEnvironmentAlreadyExists means that an environment is already created in a specific project.
type ErrEnvironmentAlreadyExists struct {
	EnvironmentName string
	ProjectName     string
}

func (e *ErrEnvironmentAlreadyExists) Error() string {
	return fmt.Sprintf("environment %s already exists in project %s",
		e.EnvironmentName, e.ProjectName)
}

// ErrNoSuchEnvironment means a specific environment couldn't be found in a specific project.
type ErrNoSuchEnvironment struct {
	ProjectName     string
	EnvironmentName string
}

func (e *ErrNoSuchEnvironment) Error() string {
	return fmt.Sprintf("couldn't find environment %s in the project %s",
		e.EnvironmentName, e.ProjectName)
}
