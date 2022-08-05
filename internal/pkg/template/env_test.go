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
				"templates/environment/cf.yml":                                 []byte("test"),
				"templates/environment/partials/cdn-resources.yml":             []byte("cdn-resources"),
				"templates/environment/partials/cfn-execution-role.yml":        []byte("cfn-execution-role"),
				"templates/environment/partials/custom-resources.yml":          []byte("custom-resources"),
				"templates/environment/partials/custom-resources-role.yml":     []byte("custom-resources-role"),
				"templates/environment/partials/environment-manager-role.yml":  []byte("environment-manager-role"),
				"templates/environment/partials/lambdas.yml":                   []byte("lambdas"),
				"templates/environment/partials/vpc-resources.yml":             []byte("vpc-resources"),
				"templates/environment/partials/nat-gateways.yml":              []byte("nat-gateways"),
				"templates/environment/partials/bootstrap-resources.yml":       []byte("bootstrap"),
				"templates/environment/partials/elb-access-logs.yml":           []byte("elb-access-logs"),
				"templates/environment/partials/mappings-regional-configs.yml": []byte("mappings-regional-configs"),
			},
		},
	}

	// WHEN
	c, err := tpl.ParseEnv(&EnvOpts{})

	// THEN
	require.NoError(t, err)
	require.Equal(t, "test", c.String())
}

func TestTemplate_ParseEnvBootstrap(t *testing.T) {
	// GIVEN
	tpl := &Template{
		fs: &mockReadFileFS{
			fs: map[string][]byte{
				"templates/environment/bootstrap-cf.yml":                      []byte("test"),
				"templates/environment/partials/cfn-execution-role.yml":       []byte("cfn-execution-role"),
				"templates/environment/partials/environment-manager-role.yml": []byte("environment-manager-role"),
				"templates/environment/partials/bootstrap-resources.yml":      []byte("bootstrap"),
			},
		},
	}

	// WHEN
	c, err := tpl.ParseEnvBootstrap(&EnvOpts{})

	// THEN
	require.NoError(t, err)
	require.Equal(t, "test", c.String())
}
