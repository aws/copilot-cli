package addon

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
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
	Property           string
	BucketNameProperty string
	ObjectKeyProperty  string

	ForceZip bool
}

var transformInfoFor = map[string]transformInfo{
	"AWS::Lambda::Function": {
		Property:           "Code",
		ForceZip:           true,
		BucketNameProperty: "S3Bucket",
		ObjectKeyProperty:  "S3Key",
	},
}

// TODO a flag to not do this on svc package
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

		info, ok := transformInfoFor[typeNode.Value]
		if !ok {
			continue
		}

		if err := a.transformProperty(propsNode, info); err != nil {
			return nil, fmt.Errorf("transform property %s property for %s: %w", name, info.Property, err)
		}
	}

	return tmpl, nil
}

func (a *Addons) transformProperty(properties *yaml.Node, tr transformInfo) error {
	props := mappingNode(properties)
	node, ok := props[tr.Property]
	if !ok || node.Kind != yaml.ScalarNode {
		// only transorm if the property is preset and a scalar node
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

	fmt.Printf("Uploaded %s as a zip to S3 at %s/%s\n", assetPath, bucket, key)

	if len(tr.BucketNameProperty) == 0 && len(tr.ObjectKeyProperty) == 0 {
		node.Value = url // TODO update to s3:// type URL, not HTTPS
		return nil
	}

	return node.Encode(map[string]string{
		tr.BucketNameProperty: bucket,
		tr.ObjectKeyProperty:  key,
	})
}

func (a *Addons) uploadAddonAsset(path string, forceZip bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	var asset asset
	if forceZip || info.IsDir() {
		asset, err = zipAsset(path)
	} else {
		asset, err = fileAsset(path)
	}
	if err != nil {
		return "", fmt.Errorf("create asset: %w", err)
	}

	// TODO copy sam logic for logging
	s3Path := artifactpath.AddonArtifact(a.wlName, asset.hash)

	url, err := a.Uploader.Upload(a.Bucket, s3Path, asset.data)
	if err != nil {
		return "", fmt.Errorf("upload %s to s3 bucket %s: %w", path, a.Bucket, err)
	}

	return url, nil
}

type asset struct {
	data io.Reader
	hash string
}

// zipAsset TODO...
func zipAsset(root string) (asset, error) {
	buf := &bytes.Buffer{}
	z := zip.NewWriter(buf)
	defer z.Close()

	hash := sha256.New()

	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}

		fname, err := filepath.Rel(root, path)
		switch {
		case err != nil:
			return fmt.Errorf("rel: %w", err)
		case fname == ".": // TODO best way to check equality?
			fname = d.Name()
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("stat: %w", err)
		}

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

// fileAsset TODO...
func fileAsset(path string) (asset, error) {
	hash := sha256.New()
	buf := &bytes.Buffer{}

	f, err := os.Open(path)
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
