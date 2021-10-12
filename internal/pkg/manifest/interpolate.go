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
	// Modified from docker/compose
	interpolatorRegexp = regexp.MustCompile(`\${([_a-zA-Z][_a-zA-Z0-9]*)}`)
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
	matches := interpolatorRegexp.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return "", nil
	}
	replaced := s
	for _, match := range matches {
		// https://pkg.go.dev/regexp#Regexp.FindAllStringSubmatch
		key := match[1]
		subTextRegexp := regexp.MustCompile(fmt.Sprintf(`\${%s}`, key))
		predefinedVal, isPredefined := i.predefinedEnvVars[key]
		if isPredefined {
			replaced = subTextRegexp.ReplaceAllString(replaced, predefinedVal)
		}
		osVal := os.Getenv(key)
		if osVal == "" {
			if isPredefined {
				continue
			}
			return "", fmt.Errorf(`environment variable "%s" is not defined`, key)
		}
		if isPredefined {
			return "", fmt.Errorf(`predefined environment variable "%s" cannot be overridden with "%s"`, key, osVal)
		}
		replaced = subTextRegexp.ReplaceAllString(replaced, osVal)
	}
	return replaced, nil
}
