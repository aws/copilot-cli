// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codestar provides a client to make API requests to AWS CodeStar Connections.
package codestar

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/codestarconnections"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type client interface {
}

// CodeStar represents a client to make requests to AWS CodeStarConnections.
type Connections struct {
	client
}

// New creates a new CloudFormation client.
func New(s *session.Session) Connections {
	return Connections{
		codestarconnections.New(s),
	}
}

// WaitForAvailableConnection blocks until the connection status has been updated from `PENDING` to `AVAILABLE` or until the max attempt window expires.
func (c *Connections) WaitForAvailableConnection(ctx context.Context, connectionARN string) error {
	err := c.client.WaitUntilUpdateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until connection %s update is complete: %w", connectionARN, err)
	}
	return nil
}

// https://docs.aws.amazon.com/codestar-connections/latest/APIReference/API_GetConnection.html
