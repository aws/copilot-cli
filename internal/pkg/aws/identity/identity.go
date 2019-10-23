// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package identity wraps AWS Security Token Service (STS) API functionality.
package identity

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// Service wraps the internal sts client.
type Service struct {
	sts stsiface.STSAPI
}

// New returns a Service configured with the input session.
func New(s *session.Session) Service {
	return Service{
		sts: sts.New(s),
	}
}

// Caller holds information about a calling entity.
type Caller struct {
	ARN     string
	Account string
	UserID  string
}

// Get returns the Caller associated with the Client's session.
func (s Service) Get() (Caller, error) {
	out, err := s.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})

	if err != nil {
		return Caller{}, fmt.Errorf("get caller identity: %w", err)
	}

	return Caller{
		ARN:     *out.Arn,
		Account: *out.Account,
		UserID:  *out.UserId,
	}, nil
}
