// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"github.com/spf13/afero"
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
	// PropertyPath is the key in a cloudformation resource's 'Properties' map to be packaged.
	// Nested properties are represented by multiple keys in the slice, so the field
	//  Properties:
	//    Code:
	//      S3: ./file-name
	// is represented by []string{"Code", "S3"}.
	PropertyPath []string

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
// https://awscli.amazonaws.com/v2/documentation/api/latest/reference/cloudformation/package.html.
var resourcePackageConfig = map[string][]packagePropertyConfig{
	"AWS::ApiGateway::RestApi": {
		{
			PropertyPath:       []string{"BodyS3Location"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
		},
	},
	"AWS::Lambda::Function": {
		{
			PropertyPath:       []string{"Code"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
			ForceZip:           true,
		},
	},
	"AWS::Lambda::LayerVersion": {
		{
			PropertyPath:       []string{"Content"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
			ForceZip:           true,
		},
	},
	"AWS::Serverless::Function": {
		{
			PropertyPath: []string{"CodeUri"},
			ForceZip:     true,
		},
	},
	"AWS::Serverless::LayerVersion": {
		{
			PropertyPath: []string{"ContentUri"},
			ForceZip:     true,
		},
	},
	"AWS::Serverless::Application": {
		{
			PropertyPath: []string{"Location"},
		},
	},
	"AWS::AppSync::GraphQLSchema": {
		{
			PropertyPath: []string{"DefinitionS3Location"},
		},
	},
	"AWS::AppSync::Resolver": {
		{
			PropertyPath: []string{"RequestMappingTemplateS3Location"},
		},
		{
			PropertyPath: []string{"ResponseMappingTemplateS3Location"},
		},
	},
	"AWS::AppSync::FunctionConfiguration": {
		{
			PropertyPath: []string{"RequestMappingTemplateS3Location"},
		},
		{
			PropertyPath: []string{"ResponseMappingTemplateS3Location"},
		},
	},
	"AWS::Serverless::Api": {
		{
			PropertyPath: []string{"DefinitionUri"},
		},
	},
	"AWS::ElasticBeanstalk::ApplicationVersion": {
		{
			PropertyPath:       []string{"SourceBundle"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
		},
	},
	"AWS::CloudFormation::Stack": {
		{
			// This implementation does not recursively package
			// the local template pointed to by TemplateURL.
			PropertyPath: []string{"TemplateURL"},
		},
	},
	"AWS::Glue::Job": {
		{
			PropertyPath: []string{"Command", "ScriptLocation"},
		},
	},
	"AWS::StepFunctions::StateMachine": {
		{
			PropertyPath:       []string{"DefinitionS3Location"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
		},
	},
	"AWS::Serverless::StateMachine": {
		{
			PropertyPath:       []string{"DefinitionUri"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
		},
	},
	"AWS::CodeCommit::Repository": {
		{
			PropertyPath:       []string{"Code", "S3"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
			ForceZip:           true,
		},
	},
}

// PackageConfig contains data needed to package a Stack.
type PackageConfig struct {
	Bucket        string
	Uploader      uploader
	WorkspacePath string
	FS            afero.Fs

	s3Path func(hash string) string
}

// Package finds references to local files in Stack's template, uploads
// the files to S3, and replaces the file path with the S3 location.
func (s *EnvironmentStack) Package(cfg PackageConfig) error {
	cfg.s3Path = artifactpath.EnvironmentAddonAsset
	return s.packageAssets(cfg)
}

// Package finds references to local files in Stack's template, uploads
// the files to S3, and replaces the file path with the S3 location.
func (s *WorkloadStack) Package(cfg PackageConfig) error {
	cfg.s3Path = func(hash string) string {
		return artifactpath.AddonAsset(s.workloadName, hash)
	}
	return s.packageAssets(cfg)
}

func (s *stack) packageAssets(cfg PackageConfig) error {
	err := cfg.packageIncludeTransforms(&s.template.Metadata, &s.template.Mappings, &s.template.Conditions, &s.template.Transform, &s.template.Resources, &s.template.Outputs)
	if err != nil {
		return fmt.Errorf("package transforms: %w", err)
	}

	// package resources
	for name, node := range mappingNode(&s.template.Resources) {
		resType := yamlMapGet(node, "Type").Value
		confs, ok := resourcePackageConfig[resType]
		if !ok {
			continue
		}

		props := yamlMapGet(node, "Properties")
		for _, conf := range confs {
			if err := cfg.packageProperty(props, conf); err != nil {
				return fmt.Errorf("package property %q of %q: %w", strings.Join(conf.PropertyPath, "."), name, err)
			}
		}
	}

	return nil
}

// packageIncludeTransforms searches each node in nodes for the CFN
// intrinsic function "Fn::Transform" with the "AWS::Include" macro. If it
// detects one, and the "Location" parameter is set to a local path, it'll
// upload those files to S3. If node is a yaml map or sequence, it will
// recursively traverse those nodes.
func (p *PackageConfig) packageIncludeTransforms(nodes ...*yaml.Node) error {
	pkg := func(node *yaml.Node) error {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}

		for key, val := range mappingNode(node) {
			switch {
			case key == "Fn::Transform":
				name := yamlMapGet(val, "Name")
				if name.Value != "AWS::Include" {
					continue
				}

				loc := yamlMapGet(yamlMapGet(val, "Parameters"), "Location")
				if !isFilePath(loc.Value) {
					continue
				}

				obj, err := p.uploadAddonAsset(loc.Value, false)
				if err != nil {
					return fmt.Errorf("upload asset: %w", err)
				}

				loc.Value = s3.Location(obj.Bucket, obj.Key)
			case val.Kind == yaml.MappingNode:
				if err := p.packageIncludeTransforms(val); err != nil {
					return err
				}
			case val.Kind == yaml.SequenceNode:
				if err := p.packageIncludeTransforms(val.Content...); err != nil {
					return err
				}
			}
		}

		return nil
	}

	for i := range nodes {
		if err := pkg(nodes[i]); err != nil {
			return err
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

func (p *PackageConfig) packageProperty(resourceProperties *yaml.Node, propCfg packagePropertyConfig) error {
	target := resourceProperties
	for _, key := range propCfg.PropertyPath {
		target = yamlMapGet(target, key)
	}

	if target.IsZero() || target.Kind != yaml.ScalarNode {
		// only transform if the node is a scalar node
		return nil
	}

	if !isFilePath(target.Value) {
		return nil
	}

	obj, err := p.uploadAddonAsset(target.Value, propCfg.ForceZip)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	if propCfg.isStringReplacement() {
		target.Value = s3.Location(obj.Bucket, obj.Key)
		return nil
	}

	return target.Encode(map[string]string{
		propCfg.BucketNameProperty: obj.Bucket,
		propCfg.ObjectKeyProperty:  obj.Key,
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

func (p *PackageConfig) uploadAddonAsset(assetPath string, forceZip bool) (template.S3ObjectLocation, error) {
	// make path absolute from wsPath
	if !filepath.IsAbs(assetPath) {
		assetPath = filepath.Join(p.WorkspacePath, assetPath)
	}

	info, err := p.FS.Stat(assetPath)
	if err != nil {
		return template.S3ObjectLocation{}, fmt.Errorf("stat: %w", err)
	}

	getAsset := p.fileAsset
	if forceZip || info.IsDir() {
		getAsset = p.zipAsset
	}
	asset, err := getAsset(assetPath)
	if err != nil {
		return template.S3ObjectLocation{}, fmt.Errorf("create asset: %w", err)
	}

	s3Path := p.s3Path(asset.hash)
	url, err := p.Uploader.Upload(p.Bucket, s3Path, asset.data)
	if err != nil {
		return template.S3ObjectLocation{}, fmt.Errorf("upload %s to s3 bucket %s: %w", assetPath, p.Bucket, err)
	}

	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return template.S3ObjectLocation{}, fmt.Errorf("parse s3 url: %w", err)
	}

	return template.S3ObjectLocation{
		Bucket: bucket,
		Key:    key,
	}, nil
}

type asset struct {
	data io.Reader
	hash string
}

// zipAsset creates an asset from the directory or file specified by root
// where the data is the compressed zip archive, and the hash is
// a hash of each files name, permission, and content. The zip file
// itself is not hashed to avoid a changing hash when non-relevant
// file metadata changes, like modification time.
func (p *PackageConfig) zipAsset(root string) (asset, error) {
	buf := &bytes.Buffer{}
	archive := zip.NewWriter(buf)
	defer archive.Close()

	hash := sha256.New()

	if err := afero.Walk(p.FS, root, func(path string, info fs.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case info.IsDir():
			return nil
		}

		fname, err := filepath.Rel(root, path)
		switch {
		case err != nil:
			return fmt.Errorf("rel: %w", err)
		case fname == ".": // happens when root == path; when a file (not a dir) is passed to `zipAsset()`
			fname = info.Name()
		}

		f, err := p.FS.Open(path)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		defer f.Close()

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("create zip file header: %w", err)
		}

		header.Name = fname
		header.Method = zip.Deflate

		zf, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip file %q: %w", fname, err)
		}

		// include the file name and permissions as part of the hash
		hash.Write([]byte(fmt.Sprintf("%s %s", fname, info.Mode().String())))
		_, err = io.Copy(io.MultiWriter(zf, hash), f)
		return err
	}); err != nil {
		return asset{}, err
	}

	return asset{
		data: buf,
		hash: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// fileAsset creates an asset from the file specified by path.
// The data is the content of the file, and the hash is the
// a hash of the file content.
func (p *PackageConfig) fileAsset(path string) (asset, error) {
	hash := sha256.New()
	buf := &bytes.Buffer{}

	f, err := p.FS.Open(path)
	if err != nil {
		return asset{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(io.MultiWriter(buf, hash), f)
	if err != nil {
		return asset{}, fmt.Errorf("copy: %w", err)
	}

	return asset{
		data: buf,
		hash: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}
