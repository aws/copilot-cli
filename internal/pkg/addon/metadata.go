package addon

import (
    "errors"
    "fmt"
    "gopkg.in/yaml.v3"
)

type Metadata struct {
    Key   string
    Value interface{}
}

// Metadata parses the Metadata section of a CloudFormation template to extract logical IDs and returns them.
func Metadatas(template string) ([]Metadata, error) {
    type cfnTemplate struct {
        Metadata yaml.Node `yaml:"Metadata"`
    }
    var tpl cfnTemplate
    if err := yaml.Unmarshal([]byte(template), &tpl); err != nil {
        return nil, fmt.Errorf("unmarshal addon cloudformation template: %w", err)
    }

    metadataNode := &tpl.Metadata
    if metadataNode.IsZero() {
        // "Metadata" is an optional field so we can skip it.
        return nil, nil
    }

    if metadataNode.Kind != yaml.MappingNode {
        return nil, errors.New(`"Metadata" field in cloudformation template is not a map`)
    }

    var metadatas []Metadata
    for _, content := range mappingContents(metadataNode) {
        metadata := Metadata{
            Key:   content.keyNode.Value,
            Value: content.valueNode.Value,
        }

        metadatas = append(metadatas, metadata)
    }
    return metadatas, nil
}