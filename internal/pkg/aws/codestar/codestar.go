// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codestar provides a client to make API requests to AWS CodeStar Connections.
package codestar

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codestarconnections"
)

var waiters = []request.WaiterOption{
	request.WithWaiterDelay(request.ConstantWaiterDelay(10 * time.Second)), // How long to wait in between poll cfn for updates.
	request.WithWaiterMaxAttempts(540),                                     // Wait for at most 90 mins for any cfn action.
}

// Connection represents a client to make requests to AWS CodeStarConnections.
type Connection struct {
	client *codestarconnections.CodeStarConnections
}

// New creates a new CloudFormation client.
func New(s *session.Session) *Connection {
	return &Connection{
		codestarconnections.New(s),
	}
}

// WaitForAvailableConnection blocks until the connection status has been updated from `PENDING` to `AVAILABLE` or until the max attempt window expires.
func (c *Connection) WaitForAvailableConnection(ctx context.Context, connectionARN string) error {
	output, err := c.client.GetConnection(
		&codestarconnections.GetConnectionInput{ConnectionArn: &connectionARN},
	)
	if err != nil {
		return fmt.Errorf("get connection info: %w", err)
	}

	context := context.WithValue(ctx, "ConnectionStatus", output.Connection.ConnectionStatus)
	err = c.waitUntilStatusAvailableWithContext(context, "ConnectionStatus", waiters...)
	if err != nil {
		return fmt.Errorf("wait until connection %s update is complete: %w", connectionARN, err)
	}
	return nil
}

func (c *Connection) waitUntilStatusAvailableWithContext(ctx aws.Context, key string, waiters ...request.WaiterOption) error {
	if ctx.Value(key) == codestarconnections.ConnectionStatusAvailable {
		return nil
	}
	return fmt.Errorf("placeholder error")
}
