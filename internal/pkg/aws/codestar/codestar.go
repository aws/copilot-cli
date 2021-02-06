// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codestar provides a client to make API requests to AWS CodeStar Connections.
package codestar

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codestarconnections"
)

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

// WaitUntilStatusAvailable blocks until the connection status has been updated from `PENDING` to `AVAILABLE` or until the max attempt window expires.
func (c *Connection) WaitUntilStatusAvailable(ctx context.Context, connectionARN string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for connection %s status to change from PENDING to AVAILABLE", connectionARN)
		case <-time.After(10 * time.Second):
			output, err := c.client.GetConnection(&codestarconnections.GetConnectionInput{ConnectionArn: aws.String(connectionARN)})
			if err != nil {
				return fmt.Errorf("get connection details: %w", err)
			}
			if *output.Connection.ConnectionStatus == codestarconnections.ConnectionStatusAvailable {
				return nil
			}
		}
	}
}
