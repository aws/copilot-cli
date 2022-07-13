// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEnv_AvailableEnvFeatures(t *testing.T) {
	c, err := New().ParseEnv(&EnvOpts{}, WithFuncs(map[string]interface{}{
		"inc":      IncFunc,
		"fmtSlice": FmtSliceFunc,
		"quote":    strconv.Quote,
	}))
	require.NoError(t, err)

	tmpl := struct {
		Params map[string]interface{} `yaml:"Parameters"`
	}{}
	b, err := c.MarshalBinary()
	require.NoError(t, err)

	err = yaml.Unmarshal(b, &tmpl)
	require.NoError(t, err)

	var exists struct{}
	featuresSet := make(map[string]struct{})
	for _, f := range AvailableEnvFeatures() {
		featuresSet[f] = exists
	}
	for paramName := range tmpl.Params {
		if !strings.HasSuffix(paramName, "Workloads") {
			continue
		}
		_, ok := featuresSet[paramName]
		require.True(t, ok, fmt.Sprintf("env-controller managed feature %s should be added as an available feature", paramName))

		_, ok = friendlyEnvFeatureName[paramName]
		require.True(t, ok, fmt.Sprintf("env-controller managed feature %s should have a friendly feature name", paramName))

		_, ok = leastVersionForFeature[paramName]
		require.True(t, ok, fmt.Sprintf("should specify a least-required environment template version for the env-controller managed feature %s", paramName))
	}
}
