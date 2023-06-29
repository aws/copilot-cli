// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines environment deployment resources.
package deploy

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	// LegacyEnvTemplateVersion is the version associated with the environment template before we started versioning.
	LegacyEnvTemplateVersion = "v0.0.0"
	// LatestEnvTemplateVersion is the latest version number available for environment templates.
	LatestEnvTemplateVersion = "v1.14.0"
	// EnvTemplateVersionBootstrap is the version of an environment template that contains only bootstrap resources.
	EnvTemplateVersionBootstrap = "bootstrap"
)

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *config.Environment
	Err error
}
