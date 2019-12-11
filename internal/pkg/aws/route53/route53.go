// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 wraps AWS route 53 API functionality.
package route53

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

// Lister interface wraps up list actions for route53 client.
type Lister interface {
	ListHostedZonesByName(in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
}

// HostedZoneExists checks if certain domain exists in any of the hosted zones.
func HostedZoneExists(hostedZones []*route53.HostedZone, domain string) bool {
	for _, hostedZone := range hostedZones {
		// example.com. should match example.com
		if domain == aws.StringValue(hostedZone.Name) || domain+"." == aws.StringValue(hostedZone.Name) {
			return true
		}
	}
	return false
}
