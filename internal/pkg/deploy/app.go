// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines app deployment resources.
package deploy

// DeleteAppInput holds the fields required to delete an application.
type DeleteAppInput struct {
	AppName     string
	EnvName     string
	ProjectName string
}
