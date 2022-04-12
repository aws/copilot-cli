// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package apprunner provides a client to make API requests to AppRunner Service.
package apprunner

import (
	"time"

	"github.com/aws/aws-sdk-go/service/apprunner"
)

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
	ImageID              string
	Port                 string
	EnvironmentVariables []*EnvironmentVariable
	Observability        ObservabilityConfiguration
}

type EnvironmentVariable struct {
	Name  string
	Value string
}

type ObservabilityConfiguration struct {
	TraceConfiguration *TraceConfiguration
}

type TraceConfiguration apprunner.TraceConfiguration
