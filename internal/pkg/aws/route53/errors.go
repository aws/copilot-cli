// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"fmt"

	"github.com/dustin/go-humanize/english"
)

// ErrDomainHostedZoneNotFound occurs when the domain hosted zone is not found.
type ErrDomainHostedZoneNotFound struct {
	domainName string
}

func (err *ErrDomainHostedZoneNotFound) Error() string {
	return fmt.Sprintf("hosted zone is not found for domain %s", err.domainName)
}

// ErrDomainNotFound occurs when the domain is not found in the account.
type ErrDomainNotFound struct {
	domainName string
}

func (err *ErrDomainNotFound) Error() string {
	return fmt.Sprintf("domain %s is not found in the account", err.domainName)
}

// ErrUnmatchedNSRecords occurs when the NS records associated with the domain do not match the name server records
// in the route53 hosted zone.
type ErrUnmatchedNSRecords struct {
	domainName   string
	hostedZoneID string
	r53Records   []string
	dnsRecords   []string
}

func (err *ErrUnmatchedNSRecords) Error() string {
	return fmt.Sprintf("name server records for %q do not match records for hosted zone %q", err.domainName, err.hostedZoneID)
}

// RecommendActions implements the main.actionRecommender interface.
func (err *ErrUnmatchedNSRecords) RecommendActions() string {
	return fmt.Sprintf(`Domain name %q has the following name server records: %s
Whereas the hosted zone ID %q for the domain has: %s
Copilot will proceed, but to use Route 53 as the DNS service, 
please ensure the name server records are mapped correctly:
- https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/migrate-dns-domain-in-use.html#migrate-dns-change-name-servers-with-provider`,
		err.domainName, english.WordSeries(err.dnsRecords, "and"),
		err.hostedZoneID, english.WordSeries(err.r53Records, "and"))
}
