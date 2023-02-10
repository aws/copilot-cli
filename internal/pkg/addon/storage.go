// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	dynamoDbTemplatePath  = "addons/ddb/cf.yml"
	s3TemplatePath        = "addons/s3/cf.yml"
	rdsTemplatePath       = "addons/aurora/cf.yml"
	rdsV2TemplatePath     = "addons/aurora/serverlessv2.yml"
	rdsRDWSTemplatePath   = "addons/aurora/rdws/cf.yml"
	rdsV2RDWSTemplatePath = "addons/aurora/rdws/serverlessv2.yml"
	rdsRDWSParamsPath     = "addons/aurora/rdws/addons.parameters.yml"

	envS3TemplatePath                   = "addons/s3/env/cf.yml"
	envS3AccessPolicyTemplatePath       = "addons/s3/env/access_policy.yml"
	envDynamoDBTemplatePath             = "addons/ddb/env/cf.yml"
	envDynamoDBAccessPolicyTemplatePath = "addons/ddb/env/access_policy.yml"
	envRDSTemplatePath                  = "addons/aurora/env/serverlessv2.yml"
	envRDSParamsPath                    = "addons/aurora/env/addons.parameters.yml"
	envRDSForRDWSTemplatePath           = "addons/aurora/env/rdws/serverlessv2.yml"
	envRDSIngressForRDWSTemplatePath    = "addons/aurora/env/rdws/ingress.yml"
	envRDSIngressForRDWSParamsPath      = "addons/aurora/env/rdws/ingress.addons.parameters.yml"
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

// StorageProps holds basic input properties for S3Props and DynamoDBProps.
type StorageProps struct {
	Name string
}

// S3Props contains S3-specific properties.
type S3Props struct {
	*StorageProps
}

// WorkloadS3Template creates a marshaler for a workload-level S3 addon.
func WorkloadS3Template(input *S3Props) *S3Template {
	return &S3Template{
		S3Props:  *input,
		parser:   template.New(),
		tmplPath: s3TemplatePath,
	}
}

// EnvS3Template creates a new marshaler for an environment-level S3 addon.
func EnvS3Template(input *S3Props) *S3Template {
	return &S3Template{
		S3Props:  *input,
		parser:   template.New(),
		tmplPath: envS3TemplatePath,
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

// DynamoDBProps contains DynamoDB-specific properties.
type DynamoDBProps struct {
	*StorageProps
	Attributes   []DDBAttribute
	LSIs         []DDBLocalSecondaryIndex
	SortKey      *string
	PartitionKey *string
	HasLSI       bool
}

// WorkloadDDBTemplate creates a marshaler for a workload-level DynamoDB addon specifying attributes,
// primary key schema, and local secondary index configuration.
func WorkloadDDBTemplate(input *DynamoDBProps) *DynamoDBTemplate {
	return &DynamoDBTemplate{
		DynamoDBProps: *input,
		parser:        template.New(),
		tmplPath:      dynamoDbTemplatePath,
	}
}

// EnvDDBTemplate creates a marshaller for an environment-level DynamoDB addon.
func EnvDDBTemplate(input *DynamoDBProps) *DynamoDBTemplate {
	return &DynamoDBTemplate{
		DynamoDBProps: *input,
		parser:        template.New(),
		tmplPath:      envDynamoDBTemplatePath,
	}
}

// DynamoDBTemplate contains configuration options which fully describe a DynamoDB table.
// Implements the encoding.BinaryMarshaler interface.
type DynamoDBTemplate struct {
	DynamoDBProps
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the template into binary.
func (d *DynamoDBTemplate) MarshalBinary() ([]byte, error) {
	content, err := d.parser.Parse(d.tmplPath, *d, template.WithFuncs(storageTemplateFunctions))
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

// AccessPolicyProps holds properties to configure an access policy to an S3 or DDB storage.
type AccessPolicyProps StorageProps

// EnvS3AccessPolicyTemplate creates a new marshaler for the access policy attached to a workload
// for permissions into an environment-level S3 addon.
func EnvS3AccessPolicyTemplate(input *AccessPolicyProps) *AccessPolicyTemplate {
	return &AccessPolicyTemplate{
		AccessPolicyProps: *input,
		parser:            template.New(),
		tmplPath:          envS3AccessPolicyTemplatePath,
	}
}

// EnvDDBAccessPolicyTemplate creates a marshaller for the access policy attached to a workload
// for permissions into an environment-level DynamoDB addon.
func EnvDDBAccessPolicyTemplate(input *AccessPolicyProps) *AccessPolicyTemplate {
	return &AccessPolicyTemplate{
		AccessPolicyProps: *input,
		parser:            template.New(),
		tmplPath:          envDynamoDBAccessPolicyTemplatePath,
	}
}

// AccessPolicyTemplate contains configuration options which describe an access policy to an S3 or DDB storage.
// Implements the encoding.BinaryMarshaler interface.
type AccessPolicyTemplate struct {
	AccessPolicyProps
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the template into binary.
func (t *AccessPolicyTemplate) MarshalBinary() ([]byte, error) {
	content, err := t.parser.Parse(t.tmplPath, *t, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// RDSProps holds RDS-specific properties.
type RDSProps struct {
	ClusterName    string   // The name of the cluster.
	Engine         string   // The engine type of the RDS Aurora Serverless cluster.
	InitialDBName  string   // The name of the initial database created inside the cluster.
	ParameterGroup string   // The parameter group to use for the cluster.
	Envs           []string // The copilot environments found inside the current app.
}

// WorkloadServerlessV1Template creates a marshaler for a workload-level Aurora Serverless v1 addon.
func WorkloadServerlessV1Template(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: rdsTemplatePath,
	}
}

// RDWSServerlessV1Template creates a marshaler for an Aurora Serverless v1 addon attached on an RDWS.
func RDWSServerlessV1Template(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: rdsRDWSTemplatePath,
	}
}

// WorkloadServerlessV2Template creates a marshaler for a workload-level Aurora Serverless v2 addon.
func WorkloadServerlessV2Template(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: rdsV2TemplatePath,
	}
}

// RDWSServerlessV2Template creates a marshaler for an Aurora Serverless v2 addon attached on an RDWS.
func RDWSServerlessV2Template(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: rdsV2RDWSTemplatePath,
	}
}

// EnvServerlessTemplate creates a marshaler for an environment-level Aurora Serverless v2 addon.
func EnvServerlessTemplate(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: envRDSTemplatePath,
	}
}

// EnvServerlessForRDWSTemplate creates a marshaler for an environment-level Aurora Serverless v2 addon
// whose ingress is an RDWS.
func EnvServerlessForRDWSTemplate(input RDSProps) *RDSTemplate {
	return &RDSTemplate{
		RDSProps: input,
		parser:   template.New(),
		tmplPath: envRDSForRDWSTemplatePath,
	}
}

// RDSIngressProps holds properties to create a security group ingress to an RDS storage.
type RDSIngressProps struct {
	ClusterName string // The name of the cluster.
	Engine      string // The engine type of the RDS Aurora Serverless cluster.
}

// EnvServerlessRDWSIngressTemplate creates a marshaler for the security group ingress attached to an RDWS
// for permissions into an environment-level Aurora Serverless v2 addon.
func EnvServerlessRDWSIngressTemplate(input RDSIngressProps) *RDSIngressTemplate {
	return &RDSIngressTemplate{
		RDSIngressProps: input,
		parser:          template.New(),
		tmplPath:        envRDSIngressForRDWSTemplatePath,
	}
}

// RDSIngressTemplate contains configuration options which describe an ingress to an RDS cluster.
// Implements the encoding.BinaryMarshaler interface.
type RDSIngressTemplate struct {
	RDSIngressProps
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the template into binary.
func (t *RDSIngressTemplate) MarshalBinary() ([]byte, error) {
	content, err := t.parser.Parse(t.tmplPath, *t, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// RDSTemplate contains configuration options which fully describe aa RDS Aurora Serverless cluster.
// Implements the encoding.BinaryMarshaler interface.
type RDSTemplate struct {
	RDSProps
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the template into binary.
func (r *RDSTemplate) MarshalBinary() ([]byte, error) {
	content, err := r.parser.Parse(r.tmplPath, *r, template.WithFuncs(storageTemplateFunctions))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// RDWSParamsForRDS creates a new RDS parameters marshaler.
func RDWSParamsForRDS() *RDSParams {
	return &RDSParams{
		parser:   template.New(),
		tmplPath: rdsRDWSParamsPath,
	}
}

// EnvParamsForRDS creates a parameter marshaler for an environment-level RDS addon.
func EnvParamsForRDS() *RDSParams {
	return &RDSParams{
		parser:   template.New(),
		tmplPath: envRDSParamsPath,
	}
}

// RDWSParamsForEnvRDS creates a parameter marshaler for the ingress attached to an RDWS
// for permissions into an environment-level RDS addon.
func RDWSParamsForEnvRDS() *RDSParams {
	return &RDSParams{
		parser:   template.New(),
		tmplPath: envRDSIngressForRDWSParamsPath,
	}
}

// RDSParams represents the addons.parameters.yml file for a RDS Aurora Serverless cluster.
type RDSParams struct {
	parser   template.Parser
	tmplPath string
}

// MarshalBinary serializes the content of the params file into binary.
func (r *RDSParams) MarshalBinary() ([]byte, error) {
	content, err := r.parser.Parse(r.tmplPath, *r, template.WithFuncs(storageTemplateFunctions))
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
