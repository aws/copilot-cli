// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
package deploy

// CreateEnvironmentInput represents the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	Project                  string // Name of the project this environment belongs to.
	Name                     string // Name of the environment, must be unique within a project.
	Prod                     bool   // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool   // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string // The Principal ARN of the tools account.
}
