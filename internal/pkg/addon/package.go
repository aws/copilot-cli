// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"fmt"
	"io"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"gopkg.in/yaml.v3"
)

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
}

// packagePropertyConfig defines how to package a particular property in a cloudformation resource.
// There are two ways replacements occur. Given a resource configuration like:
//  MyResource:
//    Type: AWS::Resource::Type
//    Properties:
//      <Property>: file/path
//
// Without BucketNameProperty and ObjectKeyProperty, `file/path` is directly replaced with
// the S3 location the contents were uploaded to, resulting in this:
//  MyResource:
//    Type: AWS::Resource::Type
//    Properties:
//      <Property>: s3://bucket/hash
//
// If BucketNameProperty and ObjectKeyProperty are set, the value of <Property> is changed to a map
// with BucketNameProperty and ObjectKeyProperty as the keys.
//  MyResource:
//    Type: AWS::Resource::Type
//    Properties:
//      <Property>:
//        <BucketNameProperty>: bucket
//        <ObjectKeyProperty>: hash
type packagePropertyConfig struct {
	// Property is the key in a cloudformation resource's 'Properties' map to be packaged.
	// Nested properties are represented by multiple keys in the slice, so the field
	//  Properties:
	//    Code:
	//      S3: ./file-name
	// is represented by []string{"Code", "S3"}.
	Property []string

	// BucketNameProperty represents the key in a submap of Property, created
	// after uploading an asset to S3. If this and ObjectKeyProperty are empty,
	// a submap will not be created and an S3 location URI will replace value of Property.
	BucketNameProperty string

	// ObjectKeyProperty represents the key in a submap of Property, created
	// after uploading an asset to S3. If this and BucketNameProperty are empty,
	// a submap will not be created and an S3 location URI will replace value of Property.
	ObjectKeyProperty string

	// ForceZip will force a zip file to be created even if the given file path
	// points to a file. Directories are always zipped.
	ForceZip bool
}

func (p *packagePropertyConfig) isStringReplacement() bool {
	return len(p.BucketNameProperty) == 0 && len(p.ObjectKeyProperty) == 0
}

// resourcePackageConfig maps a CloudFormation resource type to configuration
// for how to transform it's properties.
//
// This list of resources should stay in sync with
// https://awscli.amazonaws.com/v2/documentation/api/latest/reference/cloudformation/package.html,
// other than the AWS::Serverless resources, which are not supported in Copilot.
//
// TODO(dnrnd) AWS::Include.Location
var resourcePackageConfig = map[string][]packagePropertyConfig{
	"AWS::ApiGateway::RestApi": {
		{
			Property:           []string{"BodyS3Location"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
		},
	},
	"AWS::Lambda::Function": {
		{
			Property:           []string{"Code"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
			ForceZip:           true,
		},
	},
	"AWS::Lambda::LayerVersion": {
		{
			Property:           []string{"Content"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
			ForceZip:           true,
		},
	},
	"AWS::Serverless::Function": {
		{
			Property: []string{"CodeUri"},
			ForceZip: true,
		},
	},
	"AWS::Serverless::LayerVersion": {
		{
			Property: []string{"ContentUri"},
			ForceZip: true,
		},
	},
	"AWS::AppSync::GraphQLSchema": {
		{
			Property: []string{"DefinitionS3Location"},
		},
	},
	"AWS::AppSync::Resolver": {
		{
			Property: []string{"RequestMappingTemplateS3Location"},
		},
		{
			Property: []string{"ResponseMappingTemplateS3Location"},
		},
	},
	"AWS::AppSync::FunctionConfiguration": {
		{
			Property: []string{"RequestMappingTemplateS3Location"},
		},
		{
			Property: []string{"ResponseMappingTemplateS3Location"},
		},
	},
	"AWS::Serverless::Api": {
		{
			Property: []string{"DefinitionUri"},
		},
	},
	"AWS::ElasticBeanstalk::ApplicationVersion": {
		{
			Property:           []string{"SourceBundle"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
		},
	},
	"AWS::CloudFormation::Stack": {
		{
			// This implementation does not recursively package
			// the local template pointed to by TemplateURL.
			Property: []string{"TemplateURL"},
		},
	},
	"AWS::Glue::Job": {
		{
			Property: []string{"Command", "ScriptLocation"},
		},
	},
	"AWS::StepFunctions::StateMachine": {
		{
			Property:           []string{"DefinitionS3Location"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
		},
	},
	"AWS::Serverless::StateMachine": {
		{
			Property:           []string{"DefinitionUri"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
			ForceZip:           true,
		},
	},
	"AWS::CodeCommit::Repository": {
		{
			Property:           []string{"Code", "S3"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
			ForceZip:           true,
		},
	},
}

func (t *cfnTemplate) packageTemplate(a *Addons) error {
	resources := mappingNode(&t.Resources)

	for name, node := range resources {
		resType := yamlMapGet(node, "Type").Value
		confs, ok := resourcePackageConfig[resType]
		if !ok {
			continue
		}

		props := yamlMapGet(node, "Properties")
		for _, conf := range confs {
			if err := a.packageProperty(props, conf); err != nil {
				return fmt.Errorf("package property %q of %q: %w", strings.Join(conf.Property, "."), name, err)
			}
		}
	}

	return nil
}

// yamlMapGet parses node as a yaml map and searches key. If found,
// it returns the value node of key. If node is not a yaml MappingNode
// or key is not in the map, a zero value yaml.Node is returned, not a nil value,
// to avoid panics and simplify accessing values from the returned node.
//
// If you need access many values from yaml map, consider mappingNode() instead, as
// yamlMapGet will iterate through keys in the map each time vs a constant lookup.
func yamlMapGet(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return &yaml.Node{}
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return &yaml.Node{}
}

func (a *Addons) packageProperty(resourceProperties *yaml.Node, pkgCfg packagePropertyConfig) error {
	target := resourceProperties
	for _, key := range pkgCfg.Property {
		target = yamlMapGet(target, key)
	}

	if target.IsZero() || target.Kind != yaml.ScalarNode {
		// only transform if the node is a scalar node
		return nil
	}

	if !isFilePath(target.Value) {
		return nil
	}

	obj, err := a.uploadAddonAsset(target.Value, pkgCfg.ForceZip)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	if pkgCfg.isStringReplacement() {
		target.Value = s3.Location(obj.Bucket, obj.Key)
		return nil
	}

	return target.Encode(map[string]string{
		pkgCfg.BucketNameProperty: obj.Bucket,
		pkgCfg.ObjectKeyProperty:  obj.Key,
	})
}

// isFilePath returns true if the path URI doesn't have a
// a schema indicating it's an s3 or http URI.
func isFilePath(path string) bool {
	if path == "" || strings.HasPrefix(path, "s3://") || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}

	return true
}

type s3Object struct {
	Bucket string
	Key    string
}

func (a *Addons) uploadAddonAsset(assetPath string, forceZip bool) (s3Object, error) {
	return s3Object{
		Bucket: a.bucket,
		Key:    "TODO",
	}, nil
}
