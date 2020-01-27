// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines app deployment resources.
package deploy

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

type Database struct {
	Name     string
	Username string
	Password string
	Engine   string
}

// CreateLBFargateAppInput holds the fields required to deploy a load-balanced AWS Fargate application.
type CreateLBFargateAppInput struct {
	App          *manifest.LBFargateManifest
	Database     *Database
	Env          *archer.Environment
	ImageRepoURL string
	ImageTag     string
}
