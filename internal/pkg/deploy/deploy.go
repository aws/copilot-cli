// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
package deploy

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

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
