package addon

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
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

	var val *aOrB[yamlString, s3BucketData]
	if err := node.Decode(&val); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if val.a == "" {
		// no need to transform
		return nil
	}

	addonsDirAbs, err := a.ws.AddonsDirAbs(a.wlName)
	if err != nil {
		return fmt.Errorf("get addons directory: %w", err)
	}

	// TODO check that val.a is a local url?
	path := path.Join(addonsDirAbs, string(val.a))
	url, err := a.uploadAddonAsset(path)
	if err != nil {
		return fmt.Errorf("upload asset: %w", err)
	}

	fmt.Printf("url: %s\n", url)
	return errors.New("hello")

	// TODO upload
	// transform
	return node.Encode(val)
}

func (a *Addons) uploadAddonAsset(path string) (string, error) {
	fmt.Printf("path: %s\n", path)
	fs, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if !fs.IsDir() {
		// upload file
		return "", errors.New("TODO")
	}

	// TODO use zip and upload?

	reader, err = zipDir(path)
	if err != nil {
		return "", fmt.Errorf("zip %s: %w", path, err)
	}

	/*
		url, err := a.Uploader.Upload(a.Bucket, artifactpath.AddonArtifact(path, content), reader)
		if err != nil {
			return "", fmt.Errorf("put env file %s artifact to bucket %s: %w", path, d.resources.S3Bucket, err)
		}
	*/

	//bucket, key, err := s3.ParseURL(url)
	//if err != nil {
	//	return "", fmt.Errorf("parse s3 url: %w", err)
	//}

	return "", nil
}

func zipDir(dirPath string) (io.Reader, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	defer w.Close()

	if err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		fname, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		zf, err := w.Create(fname)
		if err != nil {
			return err
		}
		_, err = io.Copy(zf, f)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return buf, nil
}

func nodeString(n yaml.Node) string {
	b, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Addons) uploadArtifact() (s3BucketData, error) {
	return s3BucketData{}, nil
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
