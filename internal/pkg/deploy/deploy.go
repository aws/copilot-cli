// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
package deploy

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
