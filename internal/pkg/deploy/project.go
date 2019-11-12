// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines project deployment resources.
package deploy

// CreateProjectInput holds the fields required to create a project stack set.
type CreateProjectInput struct {
	Project   string // Name of the project that needs to be created.
	AccountID string // AWS account ID to administrate the project.
}

