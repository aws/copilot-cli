// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package elbv2 provides a client to make API requests to Amazon Elastic Load Balancing.
package elbv2

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

const (
	TargetHealthStateHealthy = elbv2.TargetHealthStateEnumHealthy
)

type api interface {
	DescribeTargetHealth(input *elbv2.DescribeTargetHealthInput) (*elbv2.DescribeTargetHealthOutput, error)
}

// ELBV2 wraps an AWS ELBV2 client.
type ELBV2 struct {
	client api
}

// New returns a ELBV2 configured against the input session.
func New(sess *session.Session) *ELBV2 {
	return &ELBV2{
		client: elbv2.New(sess),
	}
}

// TargetHealth wraps up elbv2.TargetHealthDescription.
type TargetHealth elbv2.TargetHealthDescription

// TargetsHealth returns the health status of the targets in a target group.
func (e *ELBV2) TargetsHealth(targetGroupARN string) ([]*TargetHealth, error) {
	in := &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupARN),
	}
	out, err := e.client.DescribeTargetHealth(in)
	if err != nil {
		return nil, fmt.Errorf("describe target health for target group %s: %w", targetGroupARN, err)
	}

	ret := make([]*TargetHealth, len(out.TargetHealthDescriptions))
	for idx, description := range out.TargetHealthDescriptions {
		ret[idx] = (*TargetHealth)(description)
	}
	return ret, nil
}

// TargetID returns the target's ID, which is either an instance or an IP address.
func (t *TargetHealth) TargetID() string {
	return aws.StringValue(t.Target.Id)
}
