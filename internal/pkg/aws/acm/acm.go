// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package acm provides a client to make API requests to AWS Certificate Manager.
package acm

import (
	"github.com/aws/aws-sdk-go/service/acm"

	"github.com/aws/aws-sdk-go/aws/session"
)

type api interface{}

// ACM wraps an AWS Certificate Manager client.
type ACM struct {
	client api
}

// New returns an ACM struct configured against the input session.
func New(s *session.Session) *ACM {
	return &ACM{
		client: acm.New(s),
	}
}

// ValidateCertAliases is a no-op and is supposed to validates if aliases are all valid against the provided ACM certificates.
func (a *ACM) ValidateCertAliases(aliases []string, certs []string) error {
	return nil
}
