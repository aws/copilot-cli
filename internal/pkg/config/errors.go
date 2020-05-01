// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import "fmt"

// ErrNoSuchApplication means an application couldn't be found within a specific account and region.
type ErrNoSuchApplication struct {
	ApplicationName string
	AccountID       string
	Region          string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchApplication) Is(target error) bool {
	t, ok := target.(*ErrNoSuchApplication)
	if !ok {
		return false
	}
	return e.ApplicationName == t.ApplicationName &&
		e.AccountID == t.AccountID &&
		e.Region == t.Region
}

func (e *ErrNoSuchApplication) Error() string {
	return fmt.Sprintf("couldn't find an application named %s in account %s and region %s",
		e.ApplicationName, e.AccountID, e.Region)
}

// ErrNoSuchEnvironment means a specific environment couldn't be found in a specific project.
type ErrNoSuchEnvironment struct {
	ApplicationName string
	EnvironmentName string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchEnvironment) Is(target error) bool {
	t, ok := target.(*ErrNoSuchEnvironment)
	if !ok {
		return false
	}
	return e.ApplicationName == t.ApplicationName &&
		e.EnvironmentName == t.EnvironmentName
}

func (e *ErrNoSuchEnvironment) Error() string {
	return fmt.Sprintf("couldn't find environment %s in the application %s",
		e.EnvironmentName, e.ApplicationName)
}

// ErrNoSuchService means a specific service couldn't be found in a specific application.
type ErrNoSuchService struct {
	ApplicationName string
	ServiceName     string
}

// Is returns whether the provided error equals this error.
func (e *ErrNoSuchService) Is(target error) bool {
	t, ok := target.(*ErrNoSuchService)
	if !ok {
		return false
	}
	return e.ApplicationName == t.ApplicationName &&
		e.ServiceName == t.ServiceName
}

func (e *ErrNoSuchService) Error() string {
	return fmt.Sprintf("couldn't find service %s in the application %s",
		e.ServiceName, e.ApplicationName)
}
