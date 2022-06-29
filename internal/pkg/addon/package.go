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

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"gopkg.in/yaml.v3"
)

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

type resource struct {
	Type       string    `yaml:"Type"`
	Properties yaml.Node `yaml:"Properties"`
}

type s3BucketData struct {
	Bucket string `yaml:"S3Bucket"`
	Key    string `yaml:"S3Key"`
}

func (s s3BucketData) IsZero() bool {
	return s.Bucket == "" && s.Key == ""
}

// TODO a flag to not do this on svc package
func (a *Addons) packageLocalArtifacts(tmpl *cfnTemplate) (*cfnTemplate, error) {
	// fmt.Printf("before resources:\n%s\n", nodeString(tmpl.Resources))
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

		switch typeNode.Value {
		case "AWS::Lambda::Function":
			if err := a.transformStringToBucketObject(propsNode, "Code"); err != nil {
				return nil, fmt.Errorf("upload and transform %q: %w", name, err)
			}
		}
	}

	// fmt.Printf("after resources:\n%s\n", nodeString(tmpl.Resources))
	return tmpl, nil
}

func (a *Addons) transformStringToBucketObject(propsNode *yaml.Node, propsKey string) error {
	props := mappingNode(propsNode)
	node, ok := props[propsKey]
	if !ok {
		return nil
	}

	var val *aOrB[*yamlPrimitive[string], s3BucketData]
	if err := node.Decode(&val); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !val.a.IsSet {
		// no need to transform
		return nil
	}

	wsPath, err := a.ws.Path() // TODO set this in New() (*Addons)?
	if err != nil {
		return fmt.Errorf("get workspace path: %w", err)
	}

	// TODO check that val.a is a local url?
	// TODO don't do if absolute path
	path := path.Join(wsPath, val.a.Value)
	url, err := a.uploadAddonAsset(path)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse s3 url: %w", err)
	}

	fmt.Printf("Uploaded %s as a zip to S3 at %s/%s\n", path, bucket, key)

	val.a.IsSet = false
	val.a.Value = ""
	val.b.Bucket = bucket
	val.b.Key = key

	return node.Encode(val)
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

type aOrB[A, B yaml.IsZeroer] struct {
	a A
	b B
}

func (a *aOrB[_, _]) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&a.a); err != nil {
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			return err
		}
	}

	if !a.a.IsZero() {
		return nil
	}

	return value.Decode(&a.b)
}

func (a *aOrB[_, _]) MarshalYAML() (interface{}, error) {
	switch {
	case !a.a.IsZero():
		return a.a, nil
	case !a.b.IsZero():
		return a.b, nil
	}
	return nil, nil
}

type yamlString string

func (y yamlString) IsZero() bool {
	return len(y) == 0
}

type yamlPrimitives interface {
	~string | ~bool | ~int | ~float64
}

type yamlPrimitive[T yamlPrimitives] struct {
	IsSet bool
	Value T
}

func (y *yamlPrimitive[T]) UnmarshalYAML(value *yaml.Node) error {
	var v T
	if err := value.Decode(&v); err != nil {
		return err
	}
	y.IsSet = true
	y.Value = v
	return nil
}

func (y *yamlPrimitive[T]) MarshalYAML() (interface{}, error) {
	if !y.IsSet {
		return nil, nil
	}
	return y.Value, nil
}

func (y *yamlPrimitive[T]) IsZero() bool {
	return !y.IsSet
}
