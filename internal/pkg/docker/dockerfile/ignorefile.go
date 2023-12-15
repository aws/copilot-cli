// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/patternmatcher/ignorefile"
)

type DockerignoreFile struct {
	excludes []string
}

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude.
func (df *DockerignoreFile) ReadDockerignore(contextDir string) error {
	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	}
	defer f.Close()

	patterns, err := ignorefile.ReadAll(f)
	if err != nil {
		return fmt.Errorf("error reading .dockerignore: %w", err)
	}

	df.excludes = patterns
	return nil
}

// Excludes returns the exclude patterns of a .dockerignore file.
func (df *DockerignoreFile) Excludes() []string {
	return df.excludes
}
