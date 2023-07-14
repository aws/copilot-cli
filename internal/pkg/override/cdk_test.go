// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

func TestCDK_Override(t *testing.T) {
	t.Parallel()
	t.Run("on install: should return a wrapped error if no package manager is installed", func(t *testing.T) {
		// GIVEN
		cdk := WithCDK("", CDKOpts{
			FS: afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "", &exec.Error{Name: "anything", Err: exec.ErrNotFound}
			},
		})

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.EqualError(t, err, `cannot find a JavaScript package manager to override with the Cloud Development Kit`)
	})
	t.Run("on install: should return a wrapped error if unexpected error occurs while finding lock file", func(t *testing.T) {
		// GIVEN
		cdk := WithCDK("", CDKOpts{
			FS: afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
		})
		cdk.exec.Find = func(_ string, _ int, _ workspace.TraverseUpProcessFn) (string, error) {
			return "", errors.New("some error")
		}

		// WHEN
		_, err := cdk.Override(nil)

		// THEN
		require.ErrorContains(t, err, `find a package lock file: some error`)
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
			ExecWriter: new(bytes.Buffer),
			FS:         mockFS,
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo")
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
			ExecWriter: new(bytes.Buffer),
			FS:         afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				if name == filepath.Join("node_modules", ".bin", "cdk") {
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
	t.Run("should invoke npm install and cdk synth, if npm is the only package manager", func(t *testing.T) {
		binPath := filepath.Join("node_modules", ".bin", "cdk")
		buf := new(strings.Builder)
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				if file == "npm" {
					return "/bin/npm", nil
				}
				return "", &exec.Error{Name: "yarn", Err: exec.ErrNotFound}
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "npm install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", binPath))
	})
	t.Run("should invoke yarn install and cdk synth, if yarn is the only package manager", func(t *testing.T) {
		binPath := filepath.Join("node_modules", ".bin", "cdk")
		buf := new(strings.Builder)
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				if file == "yarn" {
					return "/bin/yarn", nil
				}
				return "", &exec.Error{Name: "npm", Err: exec.ErrNotFound}
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "yarn install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", binPath))
	})
	t.Run("should invoke npm install and cdk synth, if only package-lock.json is found", func(t *testing.T) {
		buf := new(strings.Builder)
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("film/web/user", 0755)
		_, _ = fs.Create("film/package-lock.json")
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         fs,
			LookPathFn: func(file string) (string, error) {
				return fmt.Sprintf("/bin/%s", file), nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})
		cdk.exec.Getwd = func() (dir string, err error) {
			return "film/web/user", nil
		}

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "npm install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", filepath.Join("node_modules", ".bin", "cdk")))
	})
	t.Run("should invoke yarn install and cdk synth, if only yarn.lock is found", func(t *testing.T) {
		buf := new(strings.Builder)
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("film/web/user", 0755)
		_, _ = fs.Create("film/yarn.lock")
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         fs,
			LookPathFn: func(file string) (string, error) {
				return fmt.Sprintf("/bin/%s", file), nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})
		cdk.exec.Getwd = func() (dir string, err error) {
			return "film/web/user", nil
		}

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "yarn install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", filepath.Join("node_modules", ".bin", "cdk")))
	})
	t.Run("should invoke yarn install and cdk synth, if yarn.lock is found closer than package-lock.json", func(t *testing.T) {
		buf := new(strings.Builder)
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("/film/web/user/vip", 0755)
		_, _ = fs.Create("/film/web/user/yarn.lock")
		_, _ = fs.Create("/film/package-lock.json")
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         fs,
			LookPathFn: func(file string) (string, error) {
				return fmt.Sprintf("/bin/%s", file), nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})
		cdk.exec.Getwd = func() (dir string, err error) {
			return "/film/web/user/vip", nil
		}

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "yarn install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", filepath.Join("node_modules", ".bin", "cdk")))
	})
	t.Run("should invoke yarn install and cdk synth, if package-lock.json is found closer than yarn.lock", func(t *testing.T) {
		buf := new(strings.Builder)
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("/film/web/user/vip", 0755)
		_, _ = fs.Create("/film/web/user/package-lock.json")
		_, _ = fs.Create("/film/yarn.lock")
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         fs,
			LookPathFn: func(file string) (string, error) {
				return fmt.Sprintf("/bin/%s", file), nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})
		cdk.exec.Getwd = func() (dir string, err error) {
			return "/film/web/user/vip", nil
		}

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "npm install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", filepath.Join("node_modules", ".bin", "cdk")))
	})
	t.Run("should invoke npm install and cdk synth, if the project manager is unknown", func(t *testing.T) {
		buf := new(strings.Builder)
		fs := afero.NewMemMapFs()
		_ = fs.MkdirAll("film/web/user", 0755)
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         fs,
			LookPathFn: func(file string) (string, error) {
				return fmt.Sprintf("/bin/%s", file), nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", fmt.Sprintf("Description: %s", strings.Join(append([]string{name}, args...), " ")))
			},
		})
		cdk.exec.Getwd = func() (dir string, err error) {
			return "film/web/user", nil
		}

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)
		require.Contains(t, buf.String(), "npm install")
		require.Contains(t, string(out), fmt.Sprintf("%s synth --no-version-reporting", filepath.Join("node_modules", ".bin", "cdk")))
	})
	t.Run("should return the transformed document with CDK metadata stripped and description updated", func(t *testing.T) {
		buf := new(strings.Builder)
		cdk := WithCDK("", CDKOpts{
			ExecWriter: buf,
			FS:         afero.NewMemMapFs(),
			LookPathFn: func(file string) (string, error) {
				return "/bin/npm", nil
			},
			CommandFn: func(name string, args ...string) *exec.Cmd {
				return exec.Command("echo", `
Description: CloudFormation template that represents a load balanced web service on Amazon ECS.
AWSTemplateFormatVersion: "2010-09-09"
Metadata:
  Manifest: |
    name: admin
    type: Load Balanced Web Service
Parameters:
  AppName:
    Type: String
  BootstrapVersion:
    Type: AWS::SSM::Parameter::Value<String>
    Default: /cdk-bootstrap/hnb659fds/version
    Description: Version of the CDK Bootstrap resources in this environment, automatically retrieved from SSM Parameter Store. [cdk:skip]
Conditions:
  HasAddons:
    Fn::Not:
      - Fn::Equals:
          - Ref: AddonsTemplateURL
          - ""
Resources:
  LogGroup:
    Type: AWS::Logs::LogGroup
Outputs:
  DiscoveryServiceARN:
    Description: ARN of the Discovery Service.
Rules:
  CheckBootstrapVersion:
    Assertions:
      - Assert:
          Fn::Not:
            - Fn::Contains:
                - - "1"
                  - "2"
                  - "3"
                  - "4"
                  - "5"
                - Ref: BootstrapVersion
        AssertDescription: CDK bootstrap stack version 6 required. Please run 'cdk bootstrap' with a recent version of the CDK CLI.
`)
			},
		})

		// WHEN
		out, err := cdk.Override(nil)

		// THEN
		require.NoError(t, err)

		require.Equal(t, `AWSTemplateFormatVersion: "2010-09-09"
Description: CloudFormation template that represents a load balanced web service on Amazon ECS using AWS Copilot and CDK.
Metadata:
  Manifest: |
    name: admin
    type: Load Balanced Web Service
Parameters:
  AppName:
    Type: String
Conditions:
  HasAddons:
    Fn::Not:
      - Fn::Equals:
          - Ref: AddonsTemplateURL
          - ""
Resources:
  LogGroup:
    Type: AWS::Logs::LogGroup
Outputs:
  DiscoveryServiceARN:
    Description: ARN of the Discovery Service.
`, string(out))
	})
}

func TestScaffoldWithCDK(t *testing.T) {
	t.Run("scaffolds files in an empty directory", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		dir := filepath.Join("copilot", "frontend", "overrides")

		// WHEN
		err := ScaffoldWithCDK(fs, dir, []template.CFNResource{
			{
				Type:      "AWS::ECS::Service",
				LogicalID: "Service",
			},
		}, true)

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
	})
	t.Run("should return an error if the directory is not empty", func(t *testing.T) {
		// GIVEN
		fs := afero.NewMemMapFs()
		dir := filepath.Join("copilot", "frontend", "overrides")
		_ = fs.MkdirAll(dir, 0755)
		_ = afero.WriteFile(fs, filepath.Join(dir, "cdk.json"), []byte("content"), 0644)

		// WHEN
		err := ScaffoldWithCDK(fs, dir, nil, true)

		// THEN
		require.EqualError(t, err, fmt.Sprintf("directory %q is not empty", dir))
	})
}
