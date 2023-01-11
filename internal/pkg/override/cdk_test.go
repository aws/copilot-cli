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

type mockExec struct {
	lookPath func(file string) (string, error)
	command  func(name string, args ...string) *exec.Cmd
}

func (m *mockExec) LookPath(file string) (string, error) {
	return m.lookPath(file)
}

func (m *mockExec) Command(name string, args ...string) *exec.Cmd {
	return m.command(name, args...)
}

func TestCDK_Override(t *testing.T) {
	t.Parallel()
	t.Run("on install: should return a wrapped error if npm is not available for the users", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf(`exec: "%s": executable file not found in $PATH`, file)
			},
		}
		cdk := WithCDK("", nil, afero.NewMemMapFs(), exec)

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.EqualError(t, err, `"npm" cannot be found: "npm" is required to override with the Cloud Development Kit: exec: "npm": executable file not found in $PATH`)
	})
	t.Run("on install: should return a wrapped error if npm install fails", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("exit", "42")
			},
		}
		cdk := WithCDK("", nil, afero.NewMemMapFs(), exec)

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
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		}
		cdk := WithCDK(root, new(bytes.Buffer), mockFS, exec)

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
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				if name == filepath.Join("node_modules", "aws-cdk", "bin", "cdk") {
					return exec.Command("exit", "42")
				}
				return exec.Command("echo", "success")
			},
		}
		cdk := WithCDK("", new(bytes.Buffer), afero.NewMemMapFs(), exec)

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.ErrorContains(t, err, `run "exit 42"`)
	})
	t.Run("should invoke npm install and cdk synth", func(t *testing.T) {
		binPath := filepath.Join("node_modules", "aws-cdk", "bin", "cdk")
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		}
		buf := new(strings.Builder)
		cdk := WithCDK("", buf, afero.NewMemMapFs(), exec)

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("npm install\n%s synth --no-version-reporting\n", binPath), buf.String())
	})
	t.Run("should return the transformed document", func(t *testing.T) {
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "sample cloudformation template")
			},
		}
		buf := new(strings.Builder)
		cdk := WithCDK("", buf, afero.NewMemMapFs(), exec)

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
