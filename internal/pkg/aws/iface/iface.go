// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package iface wraps AWS SDK API functionalities.
package iface

import (
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/sts"
)

// Route53API interface wraps up API for route53 client.
type Route53API interface {
	ListHostedZonesByName(in *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
}

// ECRAPI interface wraps up API for ECR client.
type ECRAPI interface {
	GetAuthorizationToken(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)
	DescribeRepositories(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error)
	DescribeImages(*ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error)
	BatchDeleteImage(*ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error)
}

// STSAPI interface wraps up API for STS client.
type STSAPI interface {
	GetCallerIdentity(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

// ResourceGroupsTaggingAPI interface wraps up API for ResourceGroupsTagging client.
type ResourceGroupsTaggingAPI interface {
	GetResources(*resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}
