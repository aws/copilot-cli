// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

type environmentDeployer interface {
	DeployEnvironment(env *deploy.CreateEnvironmentInput) error
	StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse)
}

type projectDeployer interface {
	DeployProject(in *deploy.CreateProjectInput) error
	AddAppToProject(project *archer.Project, app *archer.Application) error
	AddEnvToProject(project *archer.Project, env *archer.Environment) error
}

type deployer interface {
	environmentDeployer
	projectDeployer
}
