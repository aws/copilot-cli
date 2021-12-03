// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"errors"
	"fmt"
	"strings"

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

// IsRegisteredDomain checks if the domain is owned by the account.
func (r *Route53Domains) IsRegisteredDomain(domainName string) error {
	_, err := r.client.GetDomainDetail(&route53domains.GetDomainDetailInput{
		DomainName: aws.String(domainName),
	})
	if err == nil {
		return nil
	}
	var errUnsupportedTLD *route53domains.UnsupportedTLD
	if errors.As(err, &errUnsupportedTLD) {
		// The TLD isn't supported by Route53, hence it can't have been registered with Route53.
		return &ErrDomainNotFound{
			domainName: domainName,
		}
	}
	var errInvalidInput *route53domains.InvalidInput
	if errors.As(err, &errInvalidInput) && strings.Contains(err.Error(), fmt.Sprintf("Domain %s not found", domainName)) {
		return &ErrDomainNotFound{
			domainName: domainName,
		}
	}
	return fmt.Errorf("get domain detail: %w", err)
}
