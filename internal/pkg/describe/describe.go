// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package describe provides information on deployed resources.
package describe

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// ResourceIdentifier is the interface to uniquely identify a resource created with the ECS CLI.
type ResourceIdentifier interface {
	URI(envName string) (string, error)
}

// NewAppIdentifier instantiates an application with a ResourceIdentifier.
func NewAppIdentifier(project, app string) (ResourceIdentifier, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetApplication(project, app)
	if err != nil {
		return nil, err
	}
	switch t := meta.Type; t {
	case manifest.LoadBalancedWebApplication:
		return newWebAppDescriber(meta, svc), nil
	default:
		return nil, fmt.Errorf("application type %s cannot be identified", t)
	}
}

type stackDescriber interface {
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
}
