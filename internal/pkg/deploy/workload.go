// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines workload deployment resources.
package deploy

const (
	// WorkloadCfnTemplateNameFormat is the base output file name when `service package`
	// or `job package` is called. This is also used to render the pipeline CFN template.
	WorkloadCfnTemplateNameFormat = "%s-%s.stack.yml"
	// WorkloadCfnTemplateConfigurationNameFormat is the base output configuration
	// file name when `service package` or `job package is called. It's also used to
	// render the pipeline CFN template.
	WorkloadCfnTemplateConfigurationNameFormat = "%s-%s.params.json"
	// AddonsCfnTemplateNameFormat is the addons output file name when `service package`
	// is called.
	AddonsCfnTemplateNameFormat = "%s.addons.stack.yml"
)

// DeleteWorkloadInput holds the fields required to delete a workload.
type DeleteWorkloadInput struct {
	Name             string
	EnvName          string
	AppName          string
	ExecutionRoleARN string
}
