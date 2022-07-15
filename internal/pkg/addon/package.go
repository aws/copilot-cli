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
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"gopkg.in/yaml.v3"
)

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
}

// transformInfo defines how to package a particular property in a cloudformation resource.
// There are two ways replacements occur. Given a resource configuration like:
//  MyResource:
//    Type: AWS::Resource::Type
//    Properties:
//      <Property>: file/path
//
// Without BucketNameProprty and ObjectKeyProperty, `file/path` is directly replaced with
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
type transformInfo struct {
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

	// ForceZip will force a zip file to be created even if the given file URI
	// points to a file. Directories are always zipped.
	ForceZip bool
}

// transformInfoFor maps CloudFormation resource types to information
// about how to transform their configuration. Some resources support
// packaging multiple properties, so they have multiple transformInfos.
//
// TODO(dnrnd) AWS::Include.Location
// TODO(dnrnd) AWS::CloudFormation::Stack
var transformInfoFor = map[string][]transformInfo{
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
	"AWS::ElasticBeanstalk::ApplicationVersion": {
		{
			Property:           []string{"SourceBundle"},
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
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
	"AWS::CodeCommit::Repository": {
		{
			Property:           []string{"Code", "S3"},
			BucketNameProperty: "Bucket",
			ObjectKeyProperty:  "Key",
			ForceZip:           true,
		},
	},
}

func (a *Addons) packageLocalArtifacts(tmpl *cfnTemplate) (*cfnTemplate, error) {
	resources := mappingNode(&tmpl.Resources)

	for name, node := range resources {
		resType := yamlMapGet(node, "Type").Value
		transforms, ok := transformInfoFor[resType]
		if !ok {
			continue
		}

		props := yamlMapGet(node, "Properties")
		for _, tr := range transforms {
			if err := a.transformProperty(props, tr); err != nil {
				return nil, fmt.Errorf("transform property %s property for %s: %w", name, tr.Property, err)
			}
		}
	}

	return tmpl, nil
}

// yamlMapGet parses node as a yaml map and searches key. If found,
// it returns the value node of key. If node is not a yaml MappingNode
// or key is not in the map, a zero value yaml.Node is returned, not a nil value,
// to make nesting easier and avoid panics.
//
// If you need access many values from yaml map, consider mappingNode() instead, as
// this will iterate through keys in the map each time vs a constant lookup.
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

func (a *Addons) transformProperty(properties *yaml.Node, tr transformInfo) error {
	node := properties
	for _, key := range tr.Property {
		node = yamlMapGet(node, key)
	}

	if node.IsZero() || node.Kind != yaml.ScalarNode {
		// only transform if the node is a scalar node
		return nil
	}

	if !isFilePath(node.Value) {
		return nil
	}

	obj, err := a.uploadAddonAsset(node.Value, tr.ForceZip)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	if len(tr.BucketNameProperty) == 0 && len(tr.ObjectKeyProperty) == 0 {
		node.Value = s3.Location(obj.Bucket, obj.Key)
		return nil
	}

	return node.Encode(map[string]string{
		tr.BucketNameProperty: obj.Bucket,
		tr.ObjectKeyProperty:  obj.Key,
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
	// make path absolute from wsPath
	if !path.IsAbs(assetPath) {
		assetPath = path.Join(a.wsPath, assetPath)
	}

	info, err := a.fs.Stat(assetPath)
	if err != nil {
		return s3Object{}, err
	}

	var asset asset
	if forceZip || info.IsDir() {
		asset, err = a.zipAsset(assetPath)
	} else {
		asset, err = a.fileAsset(assetPath)
	}
	if err != nil {
		return s3Object{}, fmt.Errorf("create asset: %w", err)
	}

	s3Path := artifactpath.AddonAsset(a.wlName, asset.hash)

	url, err := a.uploader.Upload(a.bucket, s3Path, asset.data)
	if err != nil {
		return s3Object{}, fmt.Errorf("upload %s to s3 bucket %s: %w", assetPath, a.bucket, err)
	}

	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return s3Object{}, fmt.Errorf("parse s3 url: %w", err)
	}

	fmt.Printf("Uploaded %s to s3 at: %s\n", assetPath, s3.Location(bucket, key))
	return s3Object{
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
// a hash of each of the file names, permissions, content.
func (a *Addons) zipAsset(root string) (asset, error) {
	buf := &bytes.Buffer{}
	z := zip.NewWriter(buf)
	defer z.Close()

	hash := sha256.New()

	if err := a.fs.Walk(root, func(path string, info fs.FileInfo, err error) error {
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
		case fname == ".":
			fname = info.Name()
		}

		f, err := a.fs.Open(path)
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

		zf, err := z.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip file: %w", err)
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
func (a *Addons) fileAsset(path string) (asset, error) {
	hash := sha256.New()
	buf := &bytes.Buffer{}

	f, err := a.fs.Open(path)
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
