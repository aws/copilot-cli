// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines app deployment resources.
package deploy

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

// CreateLBFargateAppInput holds the fields required to deploy a load-balanced AWS Fargate application.
type CreateLBFargateAppInput struct {
	App            *manifest.LoadBalancedWebApp
	Env            *archer.Environment
	ImageRepoURL   string
	ImageTag       string
	AdditionalTags map[string]string // AdditionalTags are labels applied to resources under the project.
}

// DeleteAppInput holds the fields required to delete an application.
type DeleteAppInput struct {
	AppName     string
	EnvName     string
	ProjectName string
}
