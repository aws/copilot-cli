// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 provides functionality to manipulate route53 primitives.
package route53

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

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
