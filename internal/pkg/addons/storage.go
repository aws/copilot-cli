// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
)

const (
	dynamoDbAddonPath = "addons/ddb/cf.yml"
	s3AddonPath       = "addons/s3/cf.yml"
)

type Storage struct {
	ResourceName *string
	Name         *string
}

type DynamoDB struct {
	Storage

	Attributes   []DDBAttribute
	LSIs         []LocalSecondaryIndex
	HasLSI       bool
	SortKey      *string
	PartitionKey *string

	parser template.Parser
}

type S3 struct {
	Storage
	parser template.Parser
}

type StorageProps struct {
	Name         string
	ResourceName string
}

type S3AddonProps struct {
	*StorageProps
}

type DynamoDbAddonProps struct {
	*StorageProps
	Attributes   []DDBAttribute
	LSIs         []LocalSecondaryIndex
	SortKey      *string
	PartitionKey *string
	HasLSI       bool
}

// DDBAttribute holds the attribute definition of a DynamoDB attribute (keys, local secondary indices)
type DDBAttribute struct {
	Name     *string
	DataType *string // Must be one of "N", "S", "B"
}

// LocalSecondaryIndex holds a representation of an LSI
type LocalSecondaryIndex struct {
	PartitionKey *string
	SortKey      *string
	Name         *string
}

// MarshalBinary serializes the DynamoDB object into a binary YAMl CF template.
// Implements the encoding.BinaryMarshaler interface.
func (d *DynamoDB) MarshalBinary() ([]byte, error) {
	content, err := d.parser.Parse(dynamoDbAddonPath, *d)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// NewDynamoDBAddon creates a DynamoDB cloudformation template specifying attributes,
// primary key schema, and local secondary index configuration
func NewDynamoDBAddon(input *DynamoDbAddonProps) *DynamoDB {
	ddbCf := &DynamoDB{}
	ddbCf.Storage = Storage{
		Name:         aws.String(input.Name),
		ResourceName: aws.String(input.ResourceName),
	}
	ddbCf.Attributes = input.Attributes
	ddbCf.LSIs = input.LSIs
	ddbCf.HasLSI = input.HasLSI
	ddbCf.PartitionKey = input.PartitionKey
	ddbCf.SortKey = input.SortKey

	ddbCf.parser = template.New()
	return ddbCf
}

// MarshalBinary serializes the S3 object into a binary YAMl CF template.
// Implements the encoding.BinaryMarshaler interface.
func (s *S3) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(s3AddonPath, *s)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// NewS3Addon creates a new S3 marshaler which can be used to write CF via addonWriter.
func NewS3Addon(input *S3AddonProps) *S3 {
	s3Cf := &S3{}
	s3Cf.Storage = Storage{
		Name:         aws.String(input.Name),
		ResourceName: aws.String(input.ResourceName),
	}

	s3Cf.parser = template.New()
	return s3Cf
}
