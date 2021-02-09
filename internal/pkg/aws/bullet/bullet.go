// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package bullet provides a client to make API requests to Fusion Service.
package bullet

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/bullet"
)

type api interface {
	DescribeService(input *bullet.DescribeServiceInput) (*bullet.DescribeServiceOutput, error)
	ListOperations(input *bullet.ListOperationsInput) (*bullet.ListOperationsOutput, error)
}

// Bullet wraps an AWS Bullet client.
type Bullet struct {
	client         api
}

// New returns a Service configured against the input session.
func New(s *session.Session) *Bullet {
	return &Bullet{
		client: bullet.New(s),
	}
}

// DescribeService calls Bullet API and returns the specified service.
func (b *Bullet) DescribeService(serviceArn string) (Service, error) {
	resp, err := b.client.DescribeService(&bullet.DescribeServiceInput{
		ServiceArn: aws.String(serviceArn),
	})
	if err != nil {
		return Service{}, fmt.Errorf("describe service %s: %w", serviceArn, err)
	}

	return Service{
		ServiceArn:   aws.StringValue(resp.Service.ServiceArn),
		Name:         aws.StringValue(resp.Service.ServiceName),
		ID:           aws.StringValue(resp.Service.ServiceId),
		Status:       aws.StringValue(resp.Service.Status),
		ServiceUrl:   aws.StringValue(resp.Service.ServiceUrl),
		DateCreated:  aws.StringValue(resp.Service.DateCreated),
		DateUpdated:  aws.StringValue(resp.Service.DateUpdated),
	}, nil
}