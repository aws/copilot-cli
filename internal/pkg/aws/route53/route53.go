// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

const (
	// See https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/DNSLimitations.html#limits-service-quotas
	// > To view limits and request higher limits for Route 53, you must change the Region to US East (N. Virginia).
	// So we have to set the region to us-east-1 to be able to find out if a domain name exists in the account.
	route53Region = "us-east-1"
)

type api interface {
	ListHostedZonesByName(in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
}

// Route53 wraps an Route53 client.
type Route53 struct {
	client api
}

// New returns a Route53 struct configured against the input session.
func New(s *session.Session) *Route53 {
	return &Route53{
		client: route53.New(s, aws.NewConfig().WithRegion(route53Region)),
	}
}

// DomainExists returns if a domain exists under a certain AWS account.
func (r *Route53) DomainExists(domainName string) (bool, error) {
	in := &route53.ListHostedZonesByNameInput{DNSName: aws.String(domainName)}
	resp, err := r.client.ListHostedZonesByName(in)
	if err != nil {
		return false, fmt.Errorf("list hosted zone for %s: %w", domainName, err)
	}
	for {
		if hostedZoneExists(resp.HostedZones, domainName) {
			return true, nil
		}
		if !aws.BoolValue(resp.IsTruncated) {
			return false, nil
		}
		in = &route53.ListHostedZonesByNameInput{DNSName: resp.NextDNSName, HostedZoneId: resp.NextHostedZoneId}
		resp, err = r.client.ListHostedZonesByName(in)
		if err != nil {
			return false, fmt.Errorf("list hosted zone for %s: %w", domainName, err)
		}
	}
}

// hostedZoneExists checks if certain domain exists in any of the hosted zones.
func hostedZoneExists(hostedZones []*route53.HostedZone, domain string) bool {
	for _, hostedZone := range hostedZones {
		// example.com. should match example.com
		if domain == aws.StringValue(hostedZone.Name) || domain+"." == aws.StringValue(hostedZone.Name) {
			return true
		}
	}
	return false
}
