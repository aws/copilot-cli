// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/patternmatcher/ignorefile"
	"github.com/spf13/afero"
)

type DockerignoreFile struct {
	excludes []string

	fs afero.Fs
}

func NewDockerignoreFile(fs afero.Fs, contextDir string) (*DockerignoreFile, error) {
	df := &DockerignoreFile{
		fs:       fs,
		excludes: []string{},
	}
	return df, df.readDockerignore(contextDir)
}

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude.
func (df *DockerignoreFile) readDockerignore(contextDir string) error {
	f, err := df.fs.Open(filepath.Join(contextDir, ".dockerignore"))
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

	if patterns != nil {
		df.excludes = patterns
	}
	return nil
}

// Excludes returns the exclude patterns of a .dockerignore file.
func (df *DockerignoreFile) Excludes() []string {
	return df.excludes
}
