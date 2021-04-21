// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package apprunner provides a client to make API requests to AppRunner Service.
package apprunner

import "time"

// Service wraps up AppRunner Service struct.
type Service struct {
	ServiceARN           string
	Name                 string
	ID                   string
	Status               string
	ServiceURL           string
	DateCreated          time.Time
	DateUpdated          time.Time
	CPU                  string
	Memory               string
	Port                 string
	EnvironmentVariables []*EnvironmentVariable
}

type EnvironmentVariable struct {
	Name  string
	Value string
}
