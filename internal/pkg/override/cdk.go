// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/spf13/afero"
)

// CDK is an Overrider that can transform a CloudFormation template with the Cloud Development Kit.
type CDK struct {
	rootAbsPath string // Absolute path to the overrides/ directory.

	execWriter io.Writer // Writer to pipe stdout and stderr content from os/exec calls.
	fs         afero.Fs  // OS file system.
	exec       struct {
		LookPath func(file string) (string, error)
		Command  func(name string, args ...string) *exec.Cmd
	} // For testing os/exec calls.
}

// CDKOpts is optional configuration for initializing a CDK Overrider.
type CDKOpts struct {
	ExecWriter io.Writer                                   // Writer to forward stdout and stderr writes from os/exec calls. If nil default to io.Discard.
	FS         afero.Fs                                    // File system interface. If nil, defaults to the OS file system.
	EnvVars    map[string]string                           // Environment variables key value pairs to pass to the "cdk synth" command.
	LookPathFn func(executable string) (string, error)     // Search for the executable under $PATH. Defaults to exec.LookPath.
	CommandFn  func(name string, args ...string) *exec.Cmd // Create a new executable command. Defaults to exec.Command rooted at the overrides/ dir.
}

// WithCDK instantiates a new CDK Overrider with root being the path to the overrides/ directory.
func WithCDK(root string, opts CDKOpts) *CDK {
	writer := io.Discard
	if opts.ExecWriter != nil {
		writer = opts.ExecWriter
	}

	fs := afero.NewOsFs()
	if opts.FS != nil {
		fs = opts.FS
	}

	lookPathFn := exec.LookPath
	if opts.LookPathFn != nil {
		lookPathFn = opts.LookPathFn
	}

	cmdFn := func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		envs, idx := make([]string, len(opts.EnvVars)), 0
		for k, v := range opts.EnvVars {
			envs[idx] = fmt.Sprintf("%s=%s", k, v)
			idx += 1
		}
		cmd.Env = append(os.Environ(), envs...)
		return cmd
	}
	if opts.CommandFn != nil {
		cmdFn = opts.CommandFn
	}

	return &CDK{
		rootAbsPath: root,
		execWriter:  writer,
		fs:          fs,
		exec: struct {
			LookPath func(file string) (string, error)
			Command  func(name string, args ...string) *exec.Cmd
		}{
			LookPath: lookPathFn,
			Command:  cmdFn,
		},
	}
}

// Override returns the extended CloudFormation template body using the CDK.
// In order to ensure the CDK transformations can be applied, Copilot first installs any CDK dependencies
// as well as the toolkit itself.
func (cdk *CDK) Override(body []byte) ([]byte, error) {
	if err := cdk.install(); err != nil {
		return nil, err
	}
	out, err := cdk.transform(body)
	if err != nil {
		return nil, err
	}
	return cdk.cleanUp(out)
}

func (cdk *CDK) install() error {
	if _, err := cdk.exec.LookPath("npm"); err != nil {
		return &errNPMUnavailable{parent: err}
	}

	cmd := cdk.exec.Command("npm", "install")
	cmd.Stdout = cdk.execWriter
	cmd.Stderr = cdk.execWriter

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return nil
}

func (cdk *CDK) transform(body []byte) ([]byte, error) {
	buildPath := filepath.Join(cdk.rootAbsPath, ".build")
	if err := cdk.fs.MkdirAll(buildPath, 0755); err != nil {
		return nil, fmt.Errorf("create %s directory to store the CloudFormation template body: %w", buildPath, err)
	}
	inputPath := filepath.Join(buildPath, "in.yml")
	if err := afero.WriteFile(cdk.fs, inputPath, body, 0644); err != nil {
		return nil, fmt.Errorf("write CloudFormation template body content at %s: %w", inputPath, err)
	}

	// We assume that a node_modules/ dir is present with the CDK downloaded after running "npm install".
	// This way clients don't need to install the CDK toolkit separately.
	cmd := cdk.exec.Command(filepath.Join("node_modules", "aws-cdk", "bin", "cdk"), "synth", "--no-version-reporting")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = cdk.execWriter
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return buf.Bytes(), nil
}

func (cdk *CDK) cleanUp(body []byte) ([]byte, error) {
	// TODO(efekarakus): Implement me.
	return body, nil
}

// ScaffoldWithCDK bootstraps a CDK application under dir/ to override the seed CloudFormation resources.
// If the directory is not empty, then returns an error.
func ScaffoldWithCDK(fs afero.Fs, dir string, seeds []template.CFNResource) error {
	// If the directory does not exist, [afero.IsEmpty] returns false and an error.
	// Therefore, we only want to check if a directory is empty only if it also exists.
	exists, _ := afero.Exists(fs, dir)
	isEmpty, _ := afero.IsEmpty(fs, dir)
	if exists && !isEmpty {
		return fmt.Errorf("directory %q is not empty", dir)
	}

	return templates.WalkOverridesCDKDir(seeds, func(name string, content *template.Content) error {
		path := filepath.Join(dir, name)
		if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("make directories along %q: %w", filepath.Dir(path), err)
		}
		if err := afero.WriteFile(fs, path, content.Bytes(), 0644); err != nil {
			return fmt.Errorf("write file at %q: %w", path, err)
		}
		return nil
	})
}
