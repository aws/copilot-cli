// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseEnv(t *testing.T) {
	// GIVEN
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("templates/environment", 0755)
	_ = afero.WriteFile(fs, "templates/environment/cf.yml", []byte("test"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/cdn-resources.yml", []byte("cdn-resources"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/cfn-execution-role.yml", []byte("cfn-execution-role"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/custom-resources.yml", []byte("custom-resources"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/custom-resources-role.yml", []byte("custom-resources-role"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/environment-manager-role.yml", []byte("environment-manager-role"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/lambdas.yml", []byte("lambdas"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/vpc-resources.yml", []byte("vpc-resources"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/nat-gateways.yml", []byte("nat-gateways"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/bootstrap-resources.yml", []byte("bootstrap"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/elb-access-logs.yml", []byte("elb-access-logs"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/mappings-regional-configs.yml", []byte("mappings-regional-configs"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/ar-vpc-connector.yml", []byte("ar-vpc-connector"), 0644)
	tpl := &Template{
		fs: &mockFS{
			Fs: fs,
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
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("templates/environment/partials", 0755)
	_ = afero.WriteFile(fs, "templates/environment/bootstrap-cf.yml", []byte("test"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/cfn-execution-role.yml", []byte("cfn-execution-role"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/environment-manager-role.yml", []byte("environment-manager-role"), 0644)
	_ = afero.WriteFile(fs, "templates/environment/partials/bootstrap-resources.yml", []byte("bootstrap"), 0644)
	tpl := &Template{
		fs: &mockFS{
			Fs: fs,
		},
	}

	// WHEN
	c, err := tpl.ParseEnvBootstrap(&EnvOpts{})

	// THEN
	require.NoError(t, err)
	require.Equal(t, "test", c.String())
}

func TestTruncate(t *testing.T) {
	tests := map[string]struct {
		s      string
		maxLen int

		expected string
	}{
		"empty string": {
			s:        "",
			maxLen:   10,
			expected: "",
		},
		"maxLen < len(string)": {
			s:        "qwerty",
			maxLen:   4,
			expected: "qwer",
		},
		"maxLen > len(string)": {
			s:        "qwerty",
			maxLen:   7,
			expected: "qwerty",
		},
		"maxLen == len(string)": {
			s:        "qwerty",
			maxLen:   6,
			expected: "qwerty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, truncate(tc.s, tc.maxLen))
		})
	}
}
