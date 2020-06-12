// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import "github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"

const (
	DynamoDBStorageType = "DynamoDB"
	S3BucketStorageType = "S3"
)

// StorageTypes are the supported storage addon types.
var StorageTypes = []string{
	DynamoDBStorageType,
	S3BucketStorageType,
}

// Storage holds data common to all storage types
type Storage struct {
	Name         string
	ResourceName string
	Type         string
	Service      string
}

type DynamoDBStorage struct {
	Storage
	DynamoDBConfig

	parser template.Parser
}

func NewDynamoDB(input *DynamoDB)
