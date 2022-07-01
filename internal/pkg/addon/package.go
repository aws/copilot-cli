package addon

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

type transformInfo struct {
	Property           string
	BucketNameProperty string
	ObjectKeyProperty  string

	ForceZip bool
}

func transformInfoFor() map[string]transformInfo {
	return map[string]transformInfo{
		"AWS::Lambda::Function": {
			Property:           "Code",
			ForceZip:           true,
			BucketNameProperty: "S3Bucket",
			ObjectKeyProperty:  "S3Key",
		},
	}
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

		info, ok := transformInfoFor()[typeNode.Value]
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
		// only transorm if it's preset and a scalar node
		return nil
	}

	if strings.HasPrefix(node.Value, "s3://") || strings.HasPrefix(node.Value, "http://") || strings.HasPrefix(node.Value, "https://") {
		return nil
	}

	assetPath := node.Value
	if !path.IsAbs(node.Value) {
		assetPath = path.Join(a.wsPath, node.Value)
	}

	url, err := a.uploadAddonAsset(assetPath)
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

func (a *Addons) uploadAddonAsset(path string) (string, error) {
	fs, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if !fs.IsDir() {
		// upload file
		return "", errors.New("TODO")
	}

	zip, err := zipDir(path)
	if err != nil {
		return "", fmt.Errorf("zip %s: %w", path, err)
	}

	// TODO copy sam logic for logging
	s3Path := artifactpath.AddonArtifact(a.wlName, zip.hash)

	url, err := a.Uploader.Upload(a.Bucket, s3Path, zip.zip)
	if err != nil {
		return "", fmt.Errorf("upload %s to s3 bucket %s: %w", path, a.Bucket, err)
	}

	return url, nil
}

type zippedDirectory struct {
	zip  io.Reader
	hash string
}

// zipDir TODO...
func zipDir(dirPath string) (zippedDirectory, error) {
	buf := &bytes.Buffer{}
	z := zip.NewWriter(buf)
	defer z.Close()

	hash := sha256.New()

	if err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case info.IsDir():
			return nil
		}

		// the file fname in the zip should be relative
		// to the directory that is being zipped
		fname, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = fname
		header.Method = zip.Deflate
		zf, err := z.CreateHeader(header)
		if err != nil {
			return err
		}

		// include the file name and permissions as part of the hash
		hash.Write([]byte(fmt.Sprintf("%s %s", fname, info.Mode().String())))

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(io.MultiWriter(zf, hash), f)
		return err
	}); err != nil {
		return zippedDirectory{}, err
	}

	return zippedDirectory{
		zip:  buf,
		hash: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}
