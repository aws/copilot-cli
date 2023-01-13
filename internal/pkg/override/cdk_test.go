// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

func TestCDK_Override(t *testing.T) {
	t.Parallel()
	t.Run("on install: should return a wrapped error if npm is not available for the users", func(t *testing.T) {
		// GIVEN
		cdk := WithCDK("", CDKOpts{
			FS: afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "", fmt.Errorf(`exec: "%s": executable file not found in $PATH`, file)
			},
		})

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.EqualError(t, err, `"npm" cannot be found: "npm" is required to override with the Cloud Development Kit: exec: "npm": executable file not found in $PATH`)
	})
	t.Run("on install: should return a wrapped error if npm install fails", func(t *testing.T) {
		// GIVEN
		cdk := WithCDK("", CDKOpts{
			FS: afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("exit", "42")
			},
		})

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.ErrorContains(t, err, `run "exit 42"`)
	})

	t.Run("should override the same hidden file on multiple Override calls", func(t *testing.T) {
		// GIVEN
		mockFS := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = mockFS.MkdirAll(root, 0755)
		mockFS = afero.NewBasePathFs(mockFS, root)
		cdk := WithCDK(root, CDKOpts{
			Stdout: new(bytes.Buffer),
			FS:     mockFS,
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		})

		// WHEN
		_, err := cdk.Override([]byte("first"))
		// THEN
		require.NoError(t, err)
		actual, _ := afero.ReadFile(mockFS, filepath.Join(root, ".build", "in.yml"))
		require.Equal(t, []byte("first"), actual, `expected to write "first" to the hidden file`)

		// WHEN
		_, err = cdk.Override([]byte("second"))
		// THEN
		require.NoError(t, err)
		actual, _ = afero.ReadFile(mockFS, filepath.Join(root, ".build", "in.yml"))
		require.Equal(t, []byte("second"), actual, `expected to write "second" to the hidden file`)
	})
	t.Run("should return a wrapped error if cdk synth fails", func(t *testing.T) {
		// GIVEN
		cdk := WithCDK("", CDKOpts{
			Stdout: new(bytes.Buffer),
			FS:     afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				if name == filepath.Join("node_modules", "aws-cdk", "bin", "cdk") {
					return exec.Command("exit", "42")
				}
				return exec.Command("echo", "success")
			},
		})

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.ErrorContains(t, err, `run "exit 42"`)
	})
	t.Run("should invoke npm install and cdk synth", func(t *testing.T) {
		binPath := filepath.Join("node_modules", "aws-cdk", "bin", "cdk")
		buf := new(strings.Builder)
		cdk := WithCDK("", CDKOpts{
			Stdout: buf,
			FS:     afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		})

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("npm install\n%s synth --no-version-reporting\n", binPath), buf.String())
	})
	t.Run("should return the transformed document", func(t *testing.T) {
		buf := new(strings.Builder)
		cdk := WithCDK("", CDKOpts{
			Stdout: buf,
			FS:     afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "sample cloudformation template")
			},
		})

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "sample cloudformation template\n", string(out))
	})
}

func TestScaffoldWithCDK(t *testing.T) {
	// GIVEN
	fs := afero.NewMemMapFs()
	dir := filepath.Join("copilot", "frontend", "overrides")

	// WHEN
	err := ScaffoldWithCDK(fs, dir, []template.CFNResource{
		{
			Type:      "AWS::ECS::Service",
			LogicalID: "Service",
		},
	})

	// THEN
	require.NoError(t, err)

	ok, _ := afero.Exists(fs, filepath.Join(dir, "package.json"))
	require.True(t, ok, "package.json should exist")

	ok, _ = afero.Exists(fs, filepath.Join(dir, "cdk.json"))
	require.True(t, ok, "cdk.json should exist")

	ok, _ = afero.Exists(fs, filepath.Join(dir, "stack.ts"))
	require.True(t, ok, "stack.ts should exist")

	ok, _ = afero.Exists(fs, filepath.Join(dir, "bin", "override.ts"))
	require.True(t, ok, "bin/override.ts should exist")
}
