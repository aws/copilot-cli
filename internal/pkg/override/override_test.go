// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spf13/afero"
)

func TestLookup(t *testing.T) {
	t.Run("should return ErrNotExist when the file path does not exist", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend")
		_ = fs.MkdirAll(root, 0755)

		// WHEN
		_, err := Lookup(filepath.Join(root, "overrides"), fs)

		// THEN
		var notExistErr *ErrNotExist
		require.ErrorAs(t, err, &notExistErr)
	})
	t.Run("should return an error when the path is a directory without a cdk.json file", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(root, 0755)

		// WHEN
		_, err := Lookup(root, fs)

		// THEN
		require.ErrorContains(t, err, `"cdk.json" does not exist`)
	})
	t.Run("should detect a CDK application if a cdk.json file exists", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(root, 0755)
		_ = afero.WriteFile(fs, filepath.Join(root, "cdk.json"), []byte("{}"), 0755)

		// WHEN
		info, err := Lookup(root, fs)

		// THEN
		require.NoError(t, err)
		require.True(t, info.IsCDK())
		require.False(t, info.IsYAMLPatch())
	})
	t.Run("should return an error when the path is a file without a YAML extension", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(root, 0755)
		_ = afero.WriteFile(fs, filepath.Join(root, "abc.js"), nil, 0755)

		// WHEN
		_, err := Lookup(filepath.Join(root, "abc.js"), fs)

		// THEN
		wantedMsg := fmt.Sprintf(`YAML patch documents require a .yml or .yaml extension: %q has a ".js" extension`, filepath.Join(root, "abc.js"))
		require.EqualError(t, err, wantedMsg)
	})
	t.Run("should return an error when the path is an empty file", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(root, 0755)
		_ = afero.WriteFile(fs, filepath.Join(root, "cfn.patch.yml"), nil, 0755)

		// WHEN
		_, err := Lookup(filepath.Join(root, "cfn.patch.yml"), fs)

		// THEN
		wantedMsg := fmt.Sprintf(`YAML patch document at %q does not contain any operations`, filepath.Join(root, "cfn.patch.yml"))
		require.EqualError(t, err, wantedMsg)
	})
	t.Run("should detect a YAML patch document on well-formed file paths", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(root, 0755)
		_ = afero.WriteFile(fs, filepath.Join(root, "cfn.patch.yml"), []byte("- {op: 5, path: '/Resources'}"), 0755)

		// WHEN
		info, err := Lookup(filepath.Join(root, "cfn.patch.yml"), fs)

		// THEN
		require.NoError(t, err)
		require.True(t, info.IsYAMLPatch())
		require.False(t, info.IsCDK())
	})
}
