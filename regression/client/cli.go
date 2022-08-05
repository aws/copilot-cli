// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

type newCLIOption func(*CLI)

// WithPath sets the path for a CLI.
func WithPath(path string) newCLIOption {
	return func(cli *CLI) {
		cli.path = path
	}
}

// NewCLI returns a wrapper around CLI.
func NewCLI(options ...newCLIOption) (*CLI, error) {
	// These tests should be run in a dockerfile so that
	// your file system and docker image repo isn't polluted
	// with test data and files. Since this is going to run
	// from Docker, the binary will be located in the root bin.
	cliPath := filepath.Join("/", "bin", "copilot")
	if os.Getenv("DRYRUN") == "true" {
		cliPath = filepath.Join("..", "..", "bin", "local", "copilot")
	}
	cli := &CLI{
		path: cliPath,
	}
	for _, opt := range options {
		opt(cli)
	}
	if _, err := os.Stat(cli.path); err != nil {
		return nil, err
	}
	return cli, nil
}

// CLI is a wrapper around os.execs.
type CLI struct {
	path       string
	workingDir string
}

func (cli *CLI) Run(commands ...string) (string, error) {
	return cli.exec(exec.Command(cli.path, commands...))
}

func (cli *CLI) exec(command *exec.Cmd) (string, error) {
	// Turn off colors
	command.Env = append(os.Environ(), "COLOR=false", "CI=true")
	command.Dir = cli.workingDir
	sess, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return "", err
	}

	contents := sess.Wait(100000000).Out.Contents()
	if exitCode := sess.ExitCode(); exitCode != 0 {
		return string(sess.Err.Contents()), fmt.Errorf("received non 0 exit code")
	}

	return string(contents), nil
}
