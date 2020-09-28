// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines service deployment resources.
package deploy

// DeleteJobInput holds the fields required to delete a service.
type DeleteJobInput struct {
	Name    string // Name of the service that needs to be deleted.
	EnvName string // Name of the environment the service is deployed in.
	AppName string // Name of the application the service belongs to.
}
