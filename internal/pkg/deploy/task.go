// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines service deployment resources.
package deploy

// CreateTaskResourcesInput holds the fields required to create a task stack.
type CreateTaskResourcesInput struct {
	Name     string
	CPU      int
	Memory   int

	Image    string
	TaskRole string
	Command  string
	EnvVars  map[string]string

	App      string
	Env      string
}
