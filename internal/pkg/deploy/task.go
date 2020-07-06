// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines service deployment resources.
package deploy

type CreateTaskResourcesInput struct {
	Name     string
	Cpu      int
	Memory   int
	Image    string
	TaskRole string
	Command  string
	EnvVars  map[string]string
}
