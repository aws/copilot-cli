// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package identity provides a client to make API requests to AWS Security Token Service.
package identity

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

type api interface {
	GetCallerIdentity(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

// STS wraps the internal sts client.
type STS struct {
	client api
}

// New returns a STS configured with the input session.
func New(s *session.Session) STS {
	return STS{
		client: sts.New(s),
	}
}

// Caller holds information about a calling entity.
type Caller struct {
	RootUserARN string
	Account     string
	UserID      string
}

// Get returns the Caller associated with the Client's session.
func (s STS) Get() (Caller, error) {
	out, err := s.client.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return Caller{}, fmt.Errorf("get caller identity: %w", err)
	}
	parsedARN, err := arn.Parse(aws.StringValue(out.Arn))
	if err != nil {
		return Caller{}, fmt.Errorf("parse caller arn: %w", err)
	}

	return Caller{
		RootUserARN: fmt.Sprintf("arn:%s:iam::%s:root", parsedARN.Partition, aws.StringValue(out.Account)),
		Account:     aws.StringValue(out.Account),
		UserID:      aws.StringValue(out.UserId),
	}, nil
}
