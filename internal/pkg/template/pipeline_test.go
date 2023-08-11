// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParsePipeline(t *testing.T) {
	// GIVEN
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("templates/environment", 0755)
	_ = afero.WriteFile(fs, "templates/cicd/pipeline_cfn.yml", []byte(`
Resources:
  {{ logicalIDSafe "this-is-not-safe" }}
  {{ isCodeStarConnection "randomSource" }}
`), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/build-action.yml", []byte("build-action"), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/role-policy-document.yml", []byte("role-policy-document"), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/role-config.yml", []byte("role-config"), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/actions.yml", []byte("actions"), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/action-config.yml", []byte("action-config"), 0644)
	_ = afero.WriteFile(fs, "templates/cicd/partials/test.yml", []byte("test"), 0644)
	tpl := &Template{
		fs: &mockFS{
			Fs: fs,
		},
	}

	// WHEN
	c, err := tpl.ParsePipeline(gomock.Any())

	// THEN
	require.NoError(t, err)
	require.Equal(t, `
Resources:
  thisDASHisDASHnotDASHsafe
  false
`, c.String())
}
