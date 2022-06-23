package addon

import (
	"errors"
	"fmt"
	"io"

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

type lambdaFunctionCode struct {
	aOrB[yamlString, s3BucketData]
}

// type lambdaFunctionCode *aOrB[yamlString, s3BucketData]

func (a *Addons) packageLocalArtifacts(tmpl *cfnTemplate) (*cfnTemplate, error) {
	fmt.Printf("before resources:\n%s\n", nodeString(tmpl.Resources))
	resources := mappingNode(&tmpl.Resources)
	fmt.Printf("resources: %#v\n", resources)

	for name := range resources {
		res := mappingNode(resources[name])
		fmt.Printf("%s: %#v\n", name, res)
		typeNode, ok := res["Type"]
		if !ok || typeNode.Kind != yaml.ScalarNode {
			continue
		}

		propsNode, ok := res["Properties"]
		if !ok || propsNode.Kind != yaml.MappingNode {
			continue
		}

		fmt.Printf("\tType: %#v\n", typeNode)
		fmt.Printf("\tProperties: %#v\n", propsNode)

		switch typeNode.Value {
		case "AWS::Lambda::Function":
			props := mappingNode(propsNode)
			fmt.Printf("\tProperties: %#v\n", props)
			codeNode, ok := props["Code"]
			fmt.Printf("\t\tCode: %#v\n", codeNode)
			if !ok {
				continue
			}

			var code lambdaFunctionCode
			if err := codeNode.Decode(&code); err != nil {
				return nil, fmt.Errorf("decode: %w", err)
			}
			fmt.Printf("\t\tCode: %#v\n", code)
			if code.a == "" {
				continue
			}

			code.b.Bucket = "s3::bucket::myBucket"
			code.b.Key = "s3::myBucket::myKey"
			code.a = ""
			if err := codeNode.Encode(code); err != nil {
				return nil, fmt.Errorf("encode: %w", err)
			}
		}
	}

	// fmt.Printf("after resources:\n%s\n", nodeString(tmpl.Resources))
	return tmpl, nil
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
	fmt.Printf("hi DANNY value unam\n")
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

func mapGet[T any](m map[string]any, keys ...string) (T, bool) {
	var zero T

	for i := range keys {
		v, ok := m[keys[i]]
		if !ok {
			return zero, false
		}

		if i+1 == len(keys) {
			vt, ok := v.(T)
			if !ok {
				return zero, false
			}
			return vt, true
		}

		m, ok = v.(map[string]any)
		if !ok {
			return zero, false
		}
	}

	return zero, false
}

func mapPut(m map[string]any, value any, keys ...string) bool {
	cur := m
	for i := range keys {
		if i+1 == len(keys) {
			cur[keys[i]] = value
		}

		/*
			sub, ok := cur[keys[i]]
			if !ok {
				cur[keys[i]] = make(map[string]any)
			}
		*/
		// not finished
	}
	return false
}
