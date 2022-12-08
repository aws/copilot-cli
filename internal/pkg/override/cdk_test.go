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

func TestCDK_Install(t *testing.T) {
	t.Parallel()
	t.Run("should return a wrapped error if npm is not available for the users", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf(`exec: "%s": executable file not found in $PATH`, file)
			},
		}
		cdk := NewCDK("", nil, afero.NewMemMapFs(), exec)

		// WHEN
		err := cdk.Install()

		// THEN
		require.EqualError(t, err, `"npm" is required to override with the Cloud Development Kit: exec: "npm": executable file not found in $PATH`)
	})
	t.Run("should return a wrapped error if npm install fails", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("exit", "42")
			},
		}
		cdk := NewCDK("", nil, afero.NewMemMapFs(), exec)

		// WHEN
		err := cdk.Install()

		// THEN
		require.ErrorContains(t, err, `run "exit 42"`)
	})
	t.Run("should pipe stdout installation results to the writer", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			lookPath: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		}
		buf := new(strings.Builder)
		cdk := NewCDK("", buf, afero.NewMemMapFs(), exec)

		// WHEN
		err := cdk.Install()

		// THEN
		require.NoError(t, err)
		require.Equal(t, "npm install\n", buf.String())
	})
}

func TestCDK_Override(t *testing.T) {
	t.Parallel()
	t.Run("should override the same hidden file on multiple Override calls", func(t *testing.T) {
		// GIVEN
		mockFS := afero.NewMemMapFs()
		root := filepath.Join("copilot", "frontend", "overrides")
		_ = mockFS.MkdirAll(root, 0755)
		mockFS = afero.NewBasePathFs(mockFS, root)
		exec := &mockExec{
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", append([]string{name}, args...)...)
			},
		}
		cdk := NewCDK(root, new(bytes.Buffer), mockFS, exec)

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
	t.Run("should return a wrapped error if override exec call fails", func(t *testing.T) {
		// GIVEN
		exec := &mockExec{
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("exit", "42")
			},
		}
		cdk := NewCDK("", new(bytes.Buffer), afero.NewMemMapFs(), exec)

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.ErrorContains(t, err, `run "exit 42"`)
	})
	t.Run("should return the transformed output", func(t *testing.T) {
		exec := &mockExec{
			command: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", "hello")
			},
		}
		cdk := NewCDK("", new(bytes.Buffer), afero.NewMemMapFs(), exec)

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "hello\n", string(out))
	})
}
