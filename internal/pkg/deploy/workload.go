// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines workload deployment resources.
package deploy

// DeleteWorkloadInput holds the fields required to delete a service.
type DeleteWorkloadInput struct {
	Name    string // Name of the workload that needs to be deleted.
	EnvName string // Name of the environment the service is deployed in.
	AppName string // Name of the application the service belongs to.
}

// DeleteJobInput holds the fields required to delete a job.
type DeleteJobInput struct {
	Name    string // Name of the job that needs to be deleted.
	EnvName string // Name of the environment the job is deployed in.
	AppName string // Name of the application the job belongs to.
}
