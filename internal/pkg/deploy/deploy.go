// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
package deploy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

// CreateEnvironmentInput holds the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	Project                  string // Name of the project this environment belongs to.
	Name                     string // Name of the environment, must be unique within a project.
	Prod                     bool   // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool   // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string // The Principal ARN of the tools account.
}

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *archer.Environment
	Err error
}

// CreateProjectInput holds the fields required to create a project stack set.
type CreateProjectInput struct {
	Project   string // Name of the project that needs to be created.
	AccountID string // AWS account ID to administrate the project.
}

// CreateLBFargateAppInput holds the fields required to deploy a load-balanced AWS Fargate application.
type CreateLBFargateAppInput struct {
	App          *manifest.LBFargateManifest
	Env          *archer.Environment
	ImageRepoURL string
	ImageTag     string
}

// Resource represents an AWS resource.
type Resource struct {
	LogicalName string
	Type        string
}

// ResourceEvent represents a status update for an AWS resource during a deployment.
type ResourceEvent struct {
	Resource
	Status       string
	StatusReason string
}
