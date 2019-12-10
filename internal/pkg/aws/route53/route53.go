// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package route53 wraps AWS route 53 API functionality.
package route53

import "github.com/aws/aws-sdk-go/service/route53"

type Route53API interface {
	ListHostedZonesByName(in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
}
