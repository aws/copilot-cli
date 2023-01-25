// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	dynamoDbTemplatePath  = "addons/ddb/cf.yml"
	s3TemplatePath        = "addons/s3/cf.yml"
	rdsTemplatePath       = "addons/aurora/cf.yml"
	rdsV2TemplatePath     = "addons/aurora/serverlessv2.yml"
	rdsRDWSTemplatePath   = "addons/aurora/rdws/cf.yml"
	rdsRDWSV2TemplatePath = "addons/aurora/rdws/serverlessv2.yml"
	rdsRDWSParamsPath     = "addons/aurora/rdws/addons.parameters.yml"

	envS3TemplatePath             = "addons/s3/env/cf.yml"
	envS3AccessPolicyTemplatePath = "addons/s3/env/access_policy.yml"
)

const (
	// Aurora Serverless versions.
	auroraServerlessVersionV1 = "v1"
	auroraServerlessVersionV2 = "v2"
)

// Engine types for RDS Aurora Serverless.
const (
	RDSEngineTypeMySQL      = "MySQL"
	RDSEngineTypePostgreSQL = "PostgreSQL"
)

var regexpMatchAttribute = regexp.MustCompile(`^(\S+):([sbnSBN])`)

var storageTemplateFunctions = map[string]interface{}{
	"logicalIDSafe": template.StripNonAlphaNumFunc,
	"envVarName":    template.EnvVarNameFunc,
	"envVarSecret":  template.EnvVarSecretFunc,
	"toSnakeCase":   template.ToSnakeCaseFunc,
}

// StorageProps holds basic input properties for addon.NewDDBTemplate() or addon.NewS3Template().
type StorageProps struct {
	Name string
}

// S3Props contains S3-specific properties for addon.NewS3Template().
type S3Props struct {
	*StorageProps
}

// NewS3Template creates a new S3 marshaler which can be used to write CF via addonWriter.
func NewS3Template(input *S3Props) *S3Template {
	return &S3Template{
		S3Props:  *input,
		parser:   template.New(),
		tmplPath: s3TemplatePath,
	}
}

// NewEnvS3Template creates a new marshaler for an environment-level S3 addon.
func NewEnvS3Template(input *S3Props) *S3Template {
	return &S3Template{
		S3Props:  *input,
		parser:   template.New(),
		tmplPath: envS3TemplatePath,
	}
}

// NewEnvS3AccessPolicyTemplate creates a new marshaler for the access policy attached to a workload
// for permissions into an environment-level S3 addon.
func NewEnvS3AccessPolicyTemplate(input *S3Props) *S3Template {
	return &S3Template{
		S3Props:  *input,
		parser:   template.New(),
		tmplPath: envS3AccessPolicyTemplatePath,
	}
}

// S3Template contains configuration options which fully describe an S3 bucket.
// Implements the encoding.BinaryMarshaler interface.
type S3Template struct {
	S3Props
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the template into binary.
func (s *S3Template) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(s.tmplPath, *s, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// DynamoDBProps contains DynamoDB-specific properties for addon.NewDDBTemplate().
type DynamoDBProps struct {
	*StorageProps
	Attributes   []DDBAttribute
	LSIs         []DDBLocalSecondaryIndex
	SortKey      *string
	PartitionKey *string
	HasLSI       bool
}

// NewDDBTemplate creates a DynamoDB cloudformation template specifying attributes,
// primary key schema, and local secondary index configuration.
func NewDDBTemplate(input *DynamoDBProps) *DynamoDBTemplate {
	return &DynamoDBTemplate{
		DynamoDBProps: *input,

		parser: template.New(),
	}
}

// DynamoDBTemplate contains configuration options which fully describe a DynamoDB table.
// Implements the encoding.BinaryMarshaler interface.
type DynamoDBTemplate struct {
	DynamoDBProps

	parser template.Parser
}

// MarshalBinary serializes the content of the template into binary.
func (d *DynamoDBTemplate) MarshalBinary() ([]byte, error) {
	content, err := d.parser.Parse(dynamoDbTemplatePath, *d, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// BuildPartitionKey generates the properties required to specify the partition key
// based on customer inputs.
func (p *DynamoDBProps) BuildPartitionKey(partitionKey string) error {
	partitionKeyAttribute, err := DDBAttributeFromKey(partitionKey)
	if err != nil {
		return err
	}
	p.Attributes = append(p.Attributes, partitionKeyAttribute)
	p.PartitionKey = partitionKeyAttribute.Name
	return nil
}

// BuildSortKey generates the correct property configuration based on customer inputs.
func (p *DynamoDBProps) BuildSortKey(noSort bool, sortKey string) (bool, error) {
	if noSort || sortKey == "" {
		return false, nil
	}
	sortKeyAttribute, err := DDBAttributeFromKey(sortKey)
	if err != nil {
		return false, err
	}
	p.Attributes = append(p.Attributes, sortKeyAttribute)
	p.SortKey = sortKeyAttribute.Name
	return true, nil
}

// BuildLocalSecondaryIndex generates the correct LocalSecondaryIndex property configuration
// based on customer input to ensure that the CF template is valid. BuildLocalSecondaryIndex
// should be called last, after BuildPartitionKey && BuildSortKey
func (p *DynamoDBProps) BuildLocalSecondaryIndex(noLSI bool, lsiSorts []string) (bool, error) {
	// If there isn't yet a partition key on the struct, we can't do anything with this call.
	if p.PartitionKey == nil {
		return false, fmt.Errorf("partition key not specified")
	}
	// If a sort key hasn't been specified, or the customer has specified that there is no LSI,
	// or there is implicitly no LSI based on the input lsiSorts value, do nothing.
	if p.SortKey == nil || noLSI || len(lsiSorts) == 0 {
		p.HasLSI = false
		return false, nil
	}
	for _, att := range lsiSorts {
		currAtt, err := DDBAttributeFromKey(att)
		if err != nil {
			return false, err
		}
		p.Attributes = append(p.Attributes, currAtt)
	}
	p.HasLSI = true
	lsiConfig, err := newLSI(*p.PartitionKey, lsiSorts)
	if err != nil {
		return false, err
	}
	p.LSIs = lsiConfig
	return true, nil
}

// DDBAttribute holds the attribute definition of a DynamoDB attribute (keys, local secondary indices).
type DDBAttribute struct {
	Name     *string
	DataType *string // Must be one of "N", "S", "B"
}

// DDBAttributeFromKey parses the DDB type and name out of keys specified in the form "Email:S"
func DDBAttributeFromKey(input string) (DDBAttribute, error) {
	attrs := regexpMatchAttribute.FindStringSubmatch(input)
	if len(attrs) == 0 {
		return DDBAttribute{}, fmt.Errorf("parse attribute from key: %s", input)
	}
	upperString := strings.ToUpper(attrs[2])
	return DDBAttribute{
		Name:     &attrs[1],
		DataType: &upperString,
	}, nil
}

// DDBLocalSecondaryIndex holds a representation of an LSI.
type DDBLocalSecondaryIndex struct {
	PartitionKey *string
	SortKey      *string
	Name         *string
}

// RDSProps holds RDS-specific properties for addon.NewRDSTemplate().
type RDSProps struct {
	WorkloadType            string   // The type of the workload associated with the RDS addon.
	ClusterName             string   // The name of the cluster.
	auroraServerlessVersion string   // The version of Aurora Serverless.
	Engine                  string   // The engine type of the RDS Aurora Serverless cluster.
	InitialDBName           string   // The name of the initial database created inside the cluster.
	ParameterGroup          string   // The parameter group to use for the cluster.
	Envs                    []string // The copilot environments found inside the current app.
}

// NewServerlessV1Template creates a new RDS marshaler which can be used to write an Aurora Serverless v1 CloudFormation template.
func NewServerlessV1Template(input RDSProps) *RDSTemplate {
	input.auroraServerlessVersion = auroraServerlessVersionV1
	return &RDSTemplate{
		RDSProps: input,

		parser: template.New(),
	}
}

// NewServerlessV2Template creates a new RDS marshaler which can be used to write an Aurora Serverless v2 CloudFormation template.
func NewServerlessV2Template(input RDSProps) *RDSTemplate {
	input.auroraServerlessVersion = auroraServerlessVersionV2
	return &RDSTemplate{
		RDSProps: input,

		parser: template.New(),
	}
}

// RDSTemplate contains configuration options which fully describe a RDS Aurora Serverless cluster.
// Implements the encoding.BinaryMarshaler interface.
type RDSTemplate struct {
	RDSProps

	parser template.Parser
}

// MarshalBinary serializes the content of the template into binary.
func (r *RDSTemplate) MarshalBinary() ([]byte, error) {
	path := rdsTemplatePath
	if r.WorkloadType != manifest.RequestDrivenWebServiceType && r.auroraServerlessVersion == auroraServerlessVersionV2 {
		path = rdsV2TemplatePath
	} else if r.WorkloadType == manifest.RequestDrivenWebServiceType && r.auroraServerlessVersion == auroraServerlessVersionV1 {
		path = rdsRDWSTemplatePath
	} else if r.WorkloadType == manifest.RequestDrivenWebServiceType && r.auroraServerlessVersion == auroraServerlessVersionV2 {
		path = rdsRDWSV2TemplatePath
	}
	content, err := r.parser.Parse(path, *r, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// NewRDSParams creates a new RDS parameters marshaler.
func NewRDSParams() *RDSParams {
	return &RDSParams{
		parser: template.New(),
	}
}

// RDSParams represents the addons.parameters.yml file for a RDS Aurora Serverless cluster.
type RDSParams struct {
	parser template.Parser
}

// MarshalBinary serializes the content of the params file into binary.
func (r *RDSParams) MarshalBinary() ([]byte, error) {
	content, err := r.parser.Parse(rdsRDWSParamsPath, *r, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func newLSI(partitionKey string, lsis []string) ([]DDBLocalSecondaryIndex, error) {
	var output []DDBLocalSecondaryIndex
	for _, lsi := range lsis {
		lsiAttr, err := DDBAttributeFromKey(lsi)
		if err != nil {
			return nil, err
		}
		output = append(output, DDBLocalSecondaryIndex{
			PartitionKey: &partitionKey,
			SortKey:      lsiAttr.Name,
			Name:         lsiAttr.Name,
		})
	}
	return output, nil
}
