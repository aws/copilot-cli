// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"errors"
	"fmt"
)

var (
	// ErrNoExistingProjects occurs when there are no projects initialized yet.
	ErrNoExistingProjects = errors.New("no existing project found, please create a project first")
)

// ErrEnvAlreadyExists occurs when an environment's name already exists under a project.
type ErrEnvAlreadyExists struct {
	Name    string
	Project string
}

func (e *ErrEnvAlreadyExists) Error() string {
	return fmt.Sprintf("environment %s already exists under project %s, please specify a different name", e.Name, e.Project)
}
