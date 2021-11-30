// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53domains"
)

type domainAPI interface {
	GetDomainDetail(input *route53domains.GetDomainDetailInput) (*route53domains.GetDomainDetailOutput, error)
}

// Route53Domains wraps an Route53Domains client.
type Route53Domains struct {
	client domainAPI
}

// New returns a Route53Domains struct configured against the input session.
func NewRoute53Domains(s *session.Session) *Route53Domains {
	return &Route53Domains{
		client: route53domains.New(s, aws.NewConfig().WithRegion(route53Region)),
	}
}

// IsDomainOwned checks if the domain is owned by the account.
func (r *Route53Domains) IsDomainOwned(domainName string) error {
	_, err := r.client.GetDomainDetail(&route53domains.GetDomainDetailInput{
		DomainName: aws.String(domainName),
	})
	if err == nil {
		return nil
	}
	var errInvalidInput *route53domains.InvalidInput
	if !errors.As(err, &errInvalidInput) {
		return fmt.Errorf("get domain detail: %w", err)
	}
	domainNotFoundRegex := regexp.MustCompile(fmt.Sprintf("Domain %s not found", domainName))
	if domainNotFoundRegex.FindString(err.Error()) == "" {
		return fmt.Errorf("get domain detail: %w", err)
	}
	return &ErrDomainNotFound{
		domainName: domainName,
	}
}
