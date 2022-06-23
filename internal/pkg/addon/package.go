package addon

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"gopkg.in/yaml.v3"
)

//type resource struct {
//	Type       string `yaml:"Type"`
//	Properties struct {
//		Code any `yaml:"Code"`
//		yaml.Node
//	} `yaml:"Properties"`
//	yaml.Node
//}

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
	ZipAndUpload(bucket, key string, files ...s3.NamedBinary) (string, error)
}

func (a *Addons) oldVer(template *cfnTemplate) (string, error) {
	// TODO upload and transform keys
	var resources map[string]resource
	if err := template.Resources.Decode(&resources); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	// fmt.Printf("resources: %#v\n", resources)

	/*
		for name, res := range resources {
			switch res.Type {
			case "AWS::Lambda::Function":
				if v, ok := res.Properties.Code.(string); ok {
					// fmt.Printf("code uri in %s: %s\n", name, v)
					// upload
					// 		resources[name].Properties.Code = any(map[string]string{})
				}
			}
		}
	*/

	return "", fmt.Errorf("bye")
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

type lambdaFunctionProperties struct {
	Code *aOrB[yamlString, s3BucketData] `yaml:"Code"`
}

type yamlAOrB[A, B yaml.IsZeroer] struct {
	Value yaml.IsZeroer
}

func (y *yamlAOrB[A, B]) IsA() bool {
	var a A
	return reflect.TypeOf(a) == reflect.TypeOf(y.Value)
}

func (y *yamlAOrB[A, B]) IsB() bool {
	var b B
	return reflect.TypeOf(b) == reflect.TypeOf(y.Value)
}

func (y *yamlAOrB[A, B]) UnmarshalYAML(value *yaml.Node) error {
	var a A
	if err := value.Decode(&a); err != nil {
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			return err
		}
	}

	if !a.IsZero() {
		y.Value = a
		return nil
	}

	var b B
	if err := value.Decode(&a); err != nil {
		return err
	}
	y.Value = b
	return nil
}

func (y *yamlAOrB[A, B]) MarshalYAML() (interface{}, error) {
	return y.Value, nil
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

func (a *Addons) packageLocalArtifacts(tmpl *cfnTemplate) (*cfnTemplate, error) {
	fmt.Printf("before resources:\n%s\n", nodeString(tmpl.Resources))
	var resources map[string]resource
	if err := tmpl.Resources.Decode(&resources); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	for name, res := range resources {
		switch res.Type {
		case "AWS::Lambda::Function":
			var props lambdaFunctionProperties
			if err := res.Properties.Decode(&props); err != nil {
				return nil, fmt.Errorf("decode properties of %s: %w", name, err)
			}
			if props.Code.a == "" {
				continue
			}

			fmt.Printf("before props %s:\n%s\n", name, nodeString(res.Properties))

			// TODO upload
			props.Code.b.Bucket = "s3::bucket::jkl;"
			props.Code.b.Key = string(props.Code.a)
			props.Code.a = ""

			if err := res.Properties.Encode(props); err != nil {
				return nil, fmt.Errorf("encode properties of %s: %w", name, err)
			}

			fmt.Printf("after props %s:\n%s\n", name, nodeString(res.Properties))
		}
	}

	fmt.Printf("after resources:\n%s\n", nodeString(tmpl.Resources))

	/*
		fmt.Printf("resources: %#v\n", tmpl.Resources)
		if tmpl.Resources.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("1")
		}

		for i := range tmpl.Resources.Content {
			if tmpl.Resources.Content[i].Kind == yaml.MappingNode {
				m := mappingNode(tmpl.Resources.Content[i])
				fmt.Printf("%v: %#v\n", i, m)
			}
		}
	*/

	return tmpl, nil
}

func nodeString(n yaml.Node) string {
	b, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(b)
}

type s3Location struct {
	Bucket string
	Key    string
}

func (a *Addons) uploadArtifact() (s3Location, error) {
	return s3Location{}, nil
}
