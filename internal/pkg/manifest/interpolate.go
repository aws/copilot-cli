// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	reservedEnvVarKeyForAppName = "COPILOT_APPLICATION_NAME"
	reservedEnvVarKeyForEnvName = "COPILOT_ENVIRONMENT_NAME"
)

var (
	// Taken from docker/compose.
	// Environment variable names consist solely of uppercase letters, digits, and underscore,
	// and do not begin with a digit. （https://pubs.opengroup.org/onlinepubs/007904875/basedefs/xbd_chap08.html）
	interpolatorEnvVarRegExp = regexp.MustCompile(`(\\?)\${([_a-zA-Z][_a-zA-Z0-9]*)}`)
)

// Interpolator substitutes variables in a manifest.
type Interpolator struct {
	predefinedEnvVars map[string]string
}

// NewInterpolator initiates a new Interpolator.
func NewInterpolator(appName, envName string) *Interpolator {
	return &Interpolator{
		predefinedEnvVars: map[string]string{
			reservedEnvVarKeyForAppName: appName,
			reservedEnvVarKeyForEnvName: envName,
		},
	}
}

// Interpolate substitutes environment variables in a string.
func (i *Interpolator) Interpolate(s string) (string, error) {
	content, err := unmarshalYAML([]byte(s))
	if err != nil {
		return "", err
	}
	if err := i.applyInterpolation(content); err != nil {
		return "", err
	}
	out, err := marshalYAML(content)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (i *Interpolator) applyInterpolation(node *yaml.Node) error {
	switch node.Tag {
	case "!!map":
		// The content of a map always come in pairs. If the node pair exists, return the map node.
		// Note that the rest of code massively uses yaml node tree.
		// Please refer to https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html
		for idx := 0; idx < len(node.Content); idx += 2 {
			if err := i.applyInterpolation(node.Content[idx+1]); err != nil {
				return err
			}
		}
	case "!!str":
		interpolated, err := i.interpolatePart(node.Value)
		if err != nil {
			return err
		}
		var s []string
		if err = json.Unmarshal([]byte(interpolated), &s); err == nil && len(s) != 0 && node.Style != yaml.LiteralStyle {
			seqNode := &yaml.Node{
				Kind: yaml.SequenceNode,
			}
			for _, value := range s {
				seqNode.Content = append(seqNode.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: value,
				})
			}
			*node = *seqNode
		} else {
			node.Value = interpolated
		}
	default:
		for _, content := range node.Content {
			if err := i.applyInterpolation(content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Interpolator) interpolatePart(s string) (string, error) {
	matches := interpolatorEnvVarRegExp.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return s, nil
	}
	replaced := s
	for _, match := range matches {
		// https://pkg.go.dev/regexp#Regexp.FindAllStringSubmatch
		key := match[2]

		if match[1] == "\\" {
			// variable is escaped (e.g. \${foo}) -> no substitution is desired, let's just remove the leading backslash
			replaced = strings.ReplaceAll(replaced, fmt.Sprintf("\\${%s}", key), fmt.Sprintf("${%s}", key))
			continue
		}

		currSegment := fmt.Sprintf("${%s}", key)
		predefinedVal, isPredefined := i.predefinedEnvVars[key]
		osVal, isEnvVarSet := os.LookupEnv(key)
		if isPredefined && isEnvVarSet && predefinedVal != osVal {
			return "", fmt.Errorf(`predefined environment variable "%s" cannot be overridden by OS environment variable with the same name`, key)
		}
		if isPredefined {
			replaced = strings.ReplaceAll(replaced, currSegment, predefinedVal)
			continue
		}
		if isEnvVarSet {
			replaced = strings.ReplaceAll(replaced, currSegment, osVal)
			continue
		}
		return "", fmt.Errorf(`environment variable "%s" is not defined`, key)
	}
	return replaced, nil
}

func unmarshalYAML(temp []byte) (*yaml.Node, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(temp, &node); err != nil {
		return nil, fmt.Errorf("unmarshal YAML template: %w", err)
	}
	return &node, nil
}

func marshalYAML(content *yaml.Node) ([]byte, error) {
	var out bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&out)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(content); err != nil {
		return nil, fmt.Errorf("marshal YAML template: %w", err)
	}
	return out.Bytes(), nil
}
