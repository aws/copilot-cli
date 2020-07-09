// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import "github.com/aws/copilot-cli/internal/pkg/config"

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	AppName                  string            // Name of the application this environment belongs to.
	Name                     string            // Name of the environment, must be unique within an application.
	Prod                     bool              // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool              // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string            // The Principal ARN of the tools account.
	AppDNSName               string            // The DNS name of this application, if it exists
	AdditionalTags           map[string]string // AdditionalTags are labels applied to resources under the application.
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
