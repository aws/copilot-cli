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

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude.
func ReadDockerignore(fs afero.Fs, contextDir string) ([]string, error) {
	f, err := fs.Open(filepath.Join(contextDir, ".dockerignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	patterns, err := ignorefile.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading .dockerignore: %w", err)
	}

	return patterns, nil
}
