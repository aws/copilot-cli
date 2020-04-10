// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
)

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	Project                  string            // Name of the project this environment belongs to.
	Name                     string            // Name of the environment, must be unique within a project.
	Prod                     bool              // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool              // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string            // The Principal ARN of the tools account.
	ProjectDNSName           string            // The DNS name of this project, if it exists
	AdditionalTags           map[string]string // AdditionalTags are labels applied to resources under the project.
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *archer.Environment
	Err error
}
