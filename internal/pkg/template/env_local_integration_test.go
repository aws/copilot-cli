// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var featureRegexp = regexp.MustCompile(`\$\{(\w+)}`) // E.g. match ${ALB} and ${EFS}.

func TestEnv_AvailableEnvFeatures(t *testing.T) {
	c, err := New().ParseEnv(&EnvOpts{})
	require.NoError(t, err)

	tmpl := struct {
		Outputs map[string]interface{} `yaml:"Outputs"`
	}{}
	b, err := c.MarshalBinary()
	require.NoError(t, err)

	err = yaml.Unmarshal(b, &tmpl)
	require.NoError(t, err)

	enabledFeaturesOutput := tmpl.Outputs["EnabledFeatures"].(map[string]interface{})
	enabledFeatures := enabledFeaturesOutput["Value"].(string)

	var exists struct{}
	featuresSet := make(map[string]struct{})
	for _, f := range AvailableEnvFeatures() {
		featuresSet[f] = exists
	}
	for _, match := range featureRegexp.FindAllStringSubmatch(enabledFeatures, -1) {
		paramName := match[1]
		_, ok := featuresSet[paramName]
		require.True(t, ok, fmt.Sprintf("env-controller managed feature %s should be added as an available feature", paramName))

		_, ok = friendlyEnvFeatureName[paramName]
		require.True(t, ok, fmt.Sprintf("env-controller managed feature %s should have a friendly feature name", paramName))

		_, ok = leastVersionForFeature[paramName]
		require.True(t, ok, fmt.Sprintf("should specify a least-required environment template version for the env-controller managed feature %s", paramName))
	}
}
