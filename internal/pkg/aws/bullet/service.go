// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package bullet provides a client to make API requests to Fusion Service.
package bullet

// Service wraps up Bullet Service struct.
type Service struct {
	ServiceArn    string
	Name          string
	ID            string
	Status        string
	DateCreated   string
	DateUpdated   string
	ServiceUrl    string
}
