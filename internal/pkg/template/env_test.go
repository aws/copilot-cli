// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseEnv(t *testing.T) {
	// GIVEN
	tpl := &Template{
		fs: &mockReadFileFS{
			fs: map[string][]byte{
				"templates/environment/cf.yml":                                []byte("test"),
				"templates/environment/partials/cfn-execution-role.yml":       []byte("cfn-execution-role"),
				"templates/environment/partials/custom-resources.yml":         []byte("custom-resources"),
				"templates/environment/partials/custom-resources-role.yml":    []byte("custom-resources-role"),
				"templates/environment/partials/environment-manager-role.yml": []byte("environment-manager-role"),
				"templates/environment/partials/lambdas.yml":                  []byte("lambdas"),
				"templates/environment/partials/vpc-resources.yml":            []byte("vpc-resources"),
				"templates/environment/partials/nat-gateways.yml":             []byte("nat-gateways"),
			},
		},
	}

	// WHEN
	c, err := tpl.ParseEnv(&EnvOpts{})

	// THEN
	require.NoError(t, err)
	require.Equal(t, "test", c.String())
}
