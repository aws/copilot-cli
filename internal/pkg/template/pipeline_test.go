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
	_ = afero.WriteFile(fs, "templates/cicd/partials/build.yml", []byte("build"), 0644)
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
