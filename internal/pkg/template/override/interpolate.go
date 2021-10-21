// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override provides functionality to replace content from vended templates.
package override

import (
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
	interpolatorEnvVarRegExp = regexp.MustCompile(`\${([_a-zA-Z][_a-zA-Z0-9]*)}`)
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
	for _, content := range node.Content {
		if err := i.applyInterpolation(content); err != nil {
			return err
		}
	}
	if node.Tag != "!!str" {
		return nil
	}
	interpolated, err := i.interpolatePart(node.Value)
	if err != nil {
		return err
	}
	node.Value = interpolated
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
		key := match[1]
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
