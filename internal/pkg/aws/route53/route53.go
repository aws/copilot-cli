// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"fmt"
	"strings"

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

// DomainHostedZoneID returns the Hosted Zone ID of a domain.
func (r *Route53) DomainHostedZoneID(domainName string) (string, error) {
	in := &route53.ListHostedZonesByNameInput{DNSName: aws.String(domainName)}
	resp, err := r.client.ListHostedZonesByName(in)
	if err != nil {
		return "", fmt.Errorf("list hosted zone for %s: %w", domainName, err)
	}
	for {
		hostedZones := filterHostedZones(resp.HostedZones, matchesDomain(domainName))
		if len(hostedZones) > 0 {
			// return the first match.
			return strings.TrimPrefix(aws.StringValue(hostedZones[0].Id), "/hostedzone/"), nil
		}
		if !aws.BoolValue(resp.IsTruncated) {
			return "", ErrDomainNotExist
		}
		in = &route53.ListHostedZonesByNameInput{DNSName: resp.NextDNSName, HostedZoneId: resp.NextHostedZoneId}
		resp, err = r.client.ListHostedZonesByName(in)
		if err != nil {
			return "", fmt.Errorf("list hosted zone for %s: %w", domainName, err)
		}
	}
}

type filterZoneFunc func(*route53.HostedZone) bool

func filterHostedZones(zones []*route53.HostedZone, fn filterZoneFunc) []*route53.HostedZone {
	var hostedZones []*route53.HostedZone
	for _, hostedZone := range zones {
		if fn(hostedZone) {
			hostedZones = append(hostedZones, hostedZone)
		}
	}
	return hostedZones
}

func matchesDomain(domain string) filterZoneFunc {
	return func(z *route53.HostedZone) bool {
		// example.com. should match example.com
		return domain == aws.StringValue(z.Name) || domain+"." == aws.StringValue(z.Name)
	}
}
