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

type transformInfo struct {
	Property           []string
	BucketNameProperty string
	ObjectKeyProperty  string

	ForceZip bool
}

// TODO(dnrnd) AWS::Include.Location
// TODO(dnrnd) AWS::CloudFormation::Stack check if valid cf template before upload, recursivly replace anything
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

	for name := range resources {
		res := mappingNode(resources[name])
		typeNode, ok := res["Type"]
		if !ok || typeNode.Kind != yaml.ScalarNode {
			continue
		}

		propsNode, ok := res["Properties"]
		if !ok || propsNode.Kind != yaml.MappingNode {
			continue
		}

		transforms, ok := transformInfoFor[typeNode.Value]
		if !ok {
			continue
		}

		for _, tr := range transforms {
			if err := a.transformProperty(propsNode, tr); err != nil {
				return nil, fmt.Errorf("transform property %s property for %s: %w", name, tr.Property, err)
			}
		}
	}

	return tmpl, nil
}

func (a *Addons) transformProperty(properties *yaml.Node, tr transformInfo) error {
	mapNode := mappingNode(properties)
	var node *yaml.Node
	for i, key := range tr.Property {
		var ok bool
		node, ok = mapNode[key]
		if !ok || (i+1 != len(tr.Property) && node.Kind != yaml.MappingNode) {
			return nil // no error if the property doesn't exist
		}
		mapNode = mappingNode(node)
	}

	if node == nil || node.Kind != yaml.ScalarNode {
		// only transform if the node is a scalar node
		return nil
	}

	if strings.HasPrefix(node.Value, "s3://") || strings.HasPrefix(node.Value, "http://") || strings.HasPrefix(node.Value, "https://") {
		return nil
	}

	assetPath := node.Value
	if !path.IsAbs(node.Value) {
		assetPath = path.Join(a.wsPath, node.Value)
	}

	url, err := a.uploadAddonAsset(assetPath, tr.ForceZip)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse s3 url: %w", err)
	}

	fmt.Printf("Uploaded %s to s3 at: %s\n", assetPath, s3.Location(bucket, key))

	if len(tr.BucketNameProperty) == 0 && len(tr.ObjectKeyProperty) == 0 {
		node.Value = s3.Location(bucket, key)
		return nil
	}

	return node.Encode(map[string]string{
		tr.BucketNameProperty: bucket,
		tr.ObjectKeyProperty:  key,
	})
}

func (a *Addons) uploadAddonAsset(path string, forceZip bool) (string, error) {
	info, err := a.fs.Stat(path)
	if err != nil {
		return "", err
	}

	var asset asset
	if forceZip || info.IsDir() {
		asset, err = a.zipAsset(path)
	} else {
		asset, err = a.fileAsset(path)
	}
	if err != nil {
		return "", fmt.Errorf("create asset: %w", err)
	}

	s3Path := artifactpath.AddonArtifact(a.wlName, asset.hash)

	url, err := a.uploader.Upload(a.bucket, s3Path, asset.data)
	if err != nil {
		return "", fmt.Errorf("upload %s to s3 bucket %s: %w", path, a.bucket, err)
	}

	return url, nil
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
		case fname == ".": // TODO best way to check equality?
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
