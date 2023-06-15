// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package cloudfront

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
)

const (
	// CertRegion is the only AWS region accepted by CloudFront while attaching certificates to a distribution.
	CertRegion = "us-east-1"

	// S3BucketOriginDomainFormat is the Regex validation format for S3 bucket as CloudFront origin domain
	// See https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-web-values-specify.html#DownloadDistValuesDomainName
	S3BucketOriginDomainFormat = `.+\.s3.*\.\w+-\w+-\d+\.amazonaws\.com`
)

type api interface {
	CreateInvalidation(input *cloudfront.CreateInvalidationInput) (*cloudfront.CreateInvalidationOutput, error)
}

// CloudFront represents a client to make requests to AWS CloudFront.
type CloudFront struct {
	client api
}

// New creates a new CloudFront client.
func New(s *session.Session) *CloudFront {
	return &CloudFront{
		client: cloudfront.New(s)}
}

// CreateInvalidation invalidates files from CloudFront edge caches before expiration.
func (c *CloudFront) CreateInvalidation(path string) error {
	output, err := c.client.CreateInvalidation(&cloudfront.CreateInvalidationInput{
		DistributionId: nil,
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: nil,
			Paths: &cloudfront.Paths{
				Items:    []*string{aws.String(path)},
				Quantity: nil,
			},
		},
	})
}
