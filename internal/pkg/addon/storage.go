// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	dynamoDbAddonPath = "addons/ddb/cf.yml"
	s3AddonPath       = "addons/s3/cf.yml"
)

var storageTemplateFunctions = map[string]interface{}{
	"logicalIDSafe": template.StripNonAlphaNumFunc,
	"envVarName":    template.EnvVarNameFunc,
}

type storage struct {
	Name *string
}

// DynamoDB contains configuration options which fully descibe a DynamoDB table.
// Implements the encoding.BinaryMarshaler interface.
type DynamoDB struct {
	DynamoDBProps

	parser template.Parser
}

// S3 contains configuration options which fully describe an S3 bucket.
// Implements the encoding.BinaryMarshaler interface.
type S3 struct {
	S3Props

	parser template.Parser
}

// StorageProps holds basic input properties for addon.NewDynamoDB() or addon.NewS3().
type StorageProps struct {
	Name string
}

// S3Props contains S3-specific properties for addon.NewS3().
type S3Props struct {
	*StorageProps
}

// DynamoDBProps contains DynamoDB-specific properties for addon.NewDynamoDB().
type DynamoDBProps struct {
	*StorageProps
	Attributes   []DDBAttribute
	LSIs         []DDBLocalSecondaryIndex
	SortKey      *string
	PartitionKey *string
	HasLSI       bool
}

// DDBAttribute holds the attribute definition of a DynamoDB attribute (keys, local secondary indices).
type DDBAttribute struct {
	Name     *string
	DataType *string // Must be one of "N", "S", "B"
}

// DDBLocalSecondaryIndex holds a representation of an LSI.
type DDBLocalSecondaryIndex struct {
	PartitionKey *string
	SortKey      *string
	Name         *string
}

// MarshalBinary serializes the DynamoDB object into a binary YAML CF template.
// Implements the encoding.BinaryMarshaler interface.
func (d *DynamoDB) MarshalBinary() ([]byte, error) {
	content, err := d.parser.Parse(dynamoDbAddonPath, *d, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// NewDynamoDB creates a DynamoDB cloudformation template specifying attributes,
// primary key schema, and local secondary index configuration.
func NewDynamoDB(input *DynamoDBProps) *DynamoDB {
	return &DynamoDB{
		DynamoDBProps: *input,

		parser: template.New(),
	}
}

// MarshalBinary serializes the S3 object into a binary YAML CF template.
// Implements the encoding.BinaryMarshaler interface.
func (s *S3) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(s3AddonPath, *s, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// NewS3 creates a new S3 marshaler which can be used to write CF via addonWriter.
func NewS3(input *S3Props) *S3 {
	return &S3{
		S3Props: *input,

		parser: template.New(),
	}
}
