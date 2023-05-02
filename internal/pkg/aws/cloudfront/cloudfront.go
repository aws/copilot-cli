// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package cloudfront

const (
	// CertRegion is the only AWS region accepted by CloudFront while attaching certificates to a distribution.
	CertRegion = "us-east-1"

	// S3BucketOriginDomainFormat is the Regex validation format for S3 bucket as CloudFront origin domain
	// See https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-web-values-specify.html#DownloadDistValuesDomainName
	S3BucketOriginDomainFormat = `.+\.s3.*\.\w+-\w+-\d+\.amazonaws\.com`
)
