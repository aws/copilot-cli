// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

const (
	// See https://docs.aws.amazon.com/general/latest/gr/r53.html
	// For Route53 API endpoint, "Route 53 in AWS Regions other than the Beijing and Ningxia Regions: specify us-east-1 as the Region."
	route53Region = "us-east-1"
)

type api interface {
	ListHostedZonesByName(*route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
	ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error)
}

type nameserverResolver interface {
	LookupNS(ctx context.Context, name string) ([]*net.NS, error)
}

// Route53 wraps an Route53 client.
type Route53 struct {
	client api
	dns    nameserverResolver

	hostedZoneIDFor map[string]string
}

// New returns a Route53 struct configured against the input session.
func New(s *session.Session) *Route53 {
	return &Route53{
		client:          route53.New(s, aws.NewConfig().WithRegion(route53Region)),
		dns:             new(net.Resolver),
		hostedZoneIDFor: make(map[string]string),
	}
}

// PublicDomainHostedZoneID returns the public Hosted Zone ID of a domain.
func (r53 *Route53) PublicDomainHostedZoneID(domainName string) (string, error) {
	if id, ok := r53.hostedZoneIDFor[domainName]; ok {
		return id, nil
	}

	in := &route53.ListHostedZonesByNameInput{DNSName: aws.String(domainName)}
	resp, err := r53.client.ListHostedZonesByName(in)
	if err != nil {
		return "", fmt.Errorf("list hosted zone for %s: %w", domainName, err)
	}
	for {
		hostedZones := filterHostedZones(resp.HostedZones, matchesDomain(domainName), matchesPublic())
		if len(hostedZones) > 0 {
			// return the first match.
			id := strings.TrimPrefix(aws.StringValue(hostedZones[0].Id), "/hostedzone/")
			r53.hostedZoneIDFor[domainName] = id
			return id, nil
		}
		if !aws.BoolValue(resp.IsTruncated) {
			return "", &ErrDomainHostedZoneNotFound{
				domainName: domainName,
			}
		}
		in = &route53.ListHostedZonesByNameInput{DNSName: resp.NextDNSName, HostedZoneId: resp.NextHostedZoneId}
		resp, err = r53.client.ListHostedZonesByName(in)
		if err != nil {
			return "", fmt.Errorf("list hosted zone for %s: %w", domainName, err)
		}
	}
}

// ValidateDomainOwnership returns nil if the NS records associated with the domain name matches the NS records of the
// route53 hosted zone for the domain.
// If there are missing NS records returns ErrUnmatchedNSRecords.
func (r53 *Route53) ValidateDomainOwnership(domainName string) error {
	hzID, err := r53.PublicDomainHostedZoneID(domainName)
	if err != nil {
		return err
	}

	wanted, err := r53.listHostedZoneNSRecords(domainName, hzID)
	if err != nil {
		return err
	}

	actual, err := r53.lookupNSRecords(domainName)
	if err != nil {
		return err
	}

	if !isStrictSubset(actual, wanted) {
		return &ErrUnmatchedNSRecords{
			domainName:   domainName,
			hostedZoneID: hzID,
			r53Records:   wanted,
			dnsRecords:   actual,
		}
	}
	return nil
}

func (r53 *Route53) listHostedZoneNSRecords(domainName, hostedZoneID string) ([]string, error) {
	out, err := r53.client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
	})
	if err != nil {
		return nil, fmt.Errorf("list resource record sets for hosted zone ID %q: %w", hostedZoneID, err)
	}
	var records []string
	for _, set := range out.ResourceRecordSets {
		if aws.StringValue(set.Type) != "NS" {
			continue
		}
		if name := aws.StringValue(set.Name); !(name == domainName || name == domainName+".") /* filter only for parent domain */ {
			continue
		}
		for _, record := range set.ResourceRecords {
			records = append(records, cleanNSRecord(aws.StringValue(record.Value)))
		}
	}
	return records, nil
}

func (r53 *Route53) lookupNSRecords(domainName string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	nameservers, err := r53.dns.LookupNS(ctx, domainName)
	if err != nil {
		return nil, fmt.Errorf("look up NS records for domain %q: %w", domainName, err)
	}

	var records []string
	for _, nameserver := range nameservers {
		records = append(records, cleanNSRecord(nameserver.Host))
	}
	return records, nil
}

type filterZoneFunc func(*route53.HostedZone) bool

func filterHostedZones(zones []*route53.HostedZone, filterFuncs ...filterZoneFunc) []*route53.HostedZone {
	var hostedZones []*route53.HostedZone
	passesAllFilters := func(zone *route53.HostedZone) bool {
		for _, fn := range filterFuncs {
			if !fn(zone) {
				return false
			}
		}
		return true
	}
	for _, hostedZone := range zones {
		if passesAllFilters(hostedZone) {
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

func matchesPublic() filterZoneFunc {
	return func(config *route53.HostedZone) bool {
		return !aws.BoolValue(config.Config.PrivateZone)
	}
}

func cleanNSRecord(record string) string {
	if !strings.HasSuffix(record, ".") {
		return record
	}
	return record[:len(record)-1]
}

func isStrictSubset(subset, superset []string) bool {
	if len(subset) > len(superset) {
		return false
	}

	isMember := make(map[string]struct{}, len(superset))
	for _, item := range superset {
		isMember[item] = struct{}{}
	}

	for _, item := range subset {
		if _, ok := isMember[item]; !ok {
			return false
		}
	}
	return true
}
