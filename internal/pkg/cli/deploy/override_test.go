// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/override"
	"github.com/stretchr/testify/require"

	"github.com/spf13/afero"
)

type mockSessProvider struct {
	UserAgent []string
}

func (m *mockSessProvider) UserAgentExtras(extras ...string) {
	m.UserAgent = append(m.UserAgent, extras...)
}

func TestNewOverrider(t *testing.T) {
	t.Run("should return override.Noop when the directory does not exist", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()

		// WHEN
		ovrdr, err := NewOverrider("overrides", "demo", "test", fs, new(mockSessProvider))

		// THEN
		require.NoError(t, err)
		_, ok := ovrdr.(*override.Noop)
		require.True(t, ok)
	})
	t.Run("should return a wrapped error when the directory is not empty but cannot be identified", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("overrides", 0755)

		// WHEN
		_, err := NewOverrider("overrides", "demo", "test", fs, new(mockSessProvider))

		// THEN
		require.ErrorContains(t, err, `look up overrider at "overrides":`)
	})
	t.Run("should initialize a CDK overrider", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("overrides", 0755)
		_ = afero.WriteFile(fs, filepath.Join("overrides", "cdk.json"), []byte(""), 0755)
		_ = afero.WriteFile(fs, filepath.Join("overrides", "package.json"), []byte(""), 0755)
		_ = afero.WriteFile(fs, filepath.Join("overrides", "stack.ts"), []byte(""), 0755)
		sess := new(mockSessProvider)

		// WHEN
		ovrdr, err := NewOverrider("overrides", "demo", "test", fs, sess)

		// THEN
		require.NoError(t, err)
		_, ok := ovrdr.(*override.CDK)
		require.True(t, ok)
		require.Contains(t, sess.UserAgent, "override cdk")
	})
}
