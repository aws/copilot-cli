// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrUnmatchedNSRecords_Error(t *testing.T) {
	err := &ErrUnmatchedNSRecords{
		domainName:   "amazon.com",
		hostedZoneID: "Z0698117FUWMJ87C39TF",
		r53Records:   []string{"ns-1119.awsdns-11.org", "ns-501.awsdns-62.com", "ns-955.awsdns-55.net", "ns-2022.awsdns-60.co.uk"},
		dnsRecords:   []string{"dns-ns2.amazon.com.", "dns-ns1.amazon.com."},
	}

	require.EqualError(t, err, `name server records for "amazon.com" do not match records for hosted zone "Z0698117FUWMJ87C39TF"`)
}

func TestErrUnmatchedNSRecords_RecommendActions(t *testing.T) {
	err := &ErrUnmatchedNSRecords{
		domainName:   "amazon.com",
		hostedZoneID: "Z0698117FUWMJ87C39TF",
		r53Records:   []string{"ns-1119.awsdns-11.org", "ns-501.awsdns-62.com", "ns-955.awsdns-55.net", "ns-2022.awsdns-60.co.uk"},
		dnsRecords:   []string{"dns-ns2.amazon.com.", "dns-ns1.amazon.com."},
	}

	require.Equal(t,
		`Domain name "amazon.com" has the following name server records: dns-ns2.amazon.com. and dns-ns1.amazon.com.
Whereas the hosted zone ID "Z0698117FUWMJ87C39TF" for the domain has: ns-1119.awsdns-11.org, ns-501.awsdns-62.com, ns-955.awsdns-55.net and ns-2022.awsdns-60.co.uk
Copilot will proceed, but to use Route 53 as the DNS service, 
please ensure the name server records are mapped correctly:
- https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/migrate-dns-domain-in-use.html#migrate-dns-change-name-servers-with-provider`,
		err.RecommendActions())
}
