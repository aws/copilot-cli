// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"os"
	"regexp"
)

const (
	predefinedEnvVarKeyForAppName = "COPILOT_APPLICATION_NAME"
	predefinedEnvVarKeyForEnvName = "COPILOT_ENVIRONMENT_NAME"
)

var (
	// Modified from docker/compose.
	// Environment variable names consist solely of uppercase letters, digits, and underscore,
	// and do not begin with a digit. （https://pubs.opengroup.org/onlinepubs/007904875/basedefs/xbd_chap08.html）
	interpolatorEnvVarRegExp = regexp.MustCompile(`\${([_a-zA-Z][_a-zA-Z0-9]*)}`)
)

type interpolator struct {
	predefinedEnvVars map[string]string
}

type predefinedEnvVar struct {
	appName string
	envName string
}

func newInterpolator(envVars predefinedEnvVar) *interpolator {
	return &interpolator{
		predefinedEnvVars: map[string]string{
			predefinedEnvVarKeyForAppName: envVars.appName,
			predefinedEnvVarKeyForEnvName: envVars.envName,
		},
	}
}

func (i *interpolator) substitute(s string) (string, error) {
	matches := interpolatorEnvVarRegExp.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return "", nil
	}
	replaced := s
	for _, match := range matches {
		// https://pkg.go.dev/regexp#Regexp.FindAllStringSubmatch
		key := match[1]
		currSegmentRegex := regexp.MustCompile(fmt.Sprintf(`\${%s}`, key))
		predefinedVal, isPredefined := i.predefinedEnvVars[key]
		osVal := os.Getenv(key)
		if isPredefined && osVal != "" {
			return "", fmt.Errorf(`predefined environment variable "%s" cannot be overridden with "%s"`, key, osVal)
		}
		if isPredefined {
			replaced = currSegmentRegex.ReplaceAllString(replaced, predefinedVal)
			continue
		}
		if osVal != "" {
			replaced = currSegmentRegex.ReplaceAllString(replaced, osVal)
			continue
		}
		return "", fmt.Errorf(`environment variable "%s" is not defined`, key)
	}
	return replaced, nil
}
