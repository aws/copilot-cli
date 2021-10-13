// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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

// Interpolator interpolates values into a manifest.
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
