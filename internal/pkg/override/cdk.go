// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"

	"github.com/spf13/afero"
)

// Executable is the interface to wrap os/exec calls.
type Executable interface {
	LookPath(file string) (string, error)
	Command(name string, args ...string) *exec.Cmd // TODO(efe): Command should set cmd.Dir = rootAbsPath.
}

// CDK is an Overrider that can transform a CloudFormation template with the Cloud Development Kit.
type CDK struct {
	rootAbsPath string // Absolute path to the overrides/ directory.

	out  io.Writer  // Writer for any os/exec calls.
	fs   afero.Fs   // OS file system.
	exec Executable // For testing os/exec calls.
}

// NewCDK instantiates a new CDK Overrider with the file system pointing ot the overrides/ dir.
func NewCDK(root string, stdout io.Writer, fs afero.Fs, exec Executable) *CDK {
	return &CDK{
		rootAbsPath: root,
		out:         stdout,
		fs:          fs,
		exec:        exec,
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
	cmd.Stdout = cdk.out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return nil
}

func (cdk *CDK) transform(body []byte) ([]byte, error) {
	buildPath := filepath.Join(cdk.rootAbsPath, ".build")
	if err := cdk.fs.MkdirAll(buildPath, fs.ModeDir); err != nil {
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
	cmd.Stdout = io.MultiWriter(buf, cdk.out)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(`run %q: %w`, cmd.String(), err)
	}
	return buf.Bytes(), nil
}

func (cdk *CDK) cleanUp(body []byte) ([]byte, error) {
	// TODO(efekarakus): Implement me.
	return body, nil
}
