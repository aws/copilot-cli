// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/afero"
)

// listDockerfiles returns the list of Dockerfiles within the current
// working directory and a sub-directory level below. If an error occurs while
// reading directories, or no Dockerfiles found returns the error.
func listDockerfiles(fs afero.Fs) ([]string, error) {
	wdFiles, err := afero.ReadDir(fs, ".")
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var directories []string

	for _, wdFile := range wdFiles {
		// Add current directory if a Dockerfile exists, otherwise continue.
		if !wdFile.IsDir() {
			if wdFile.Name() == "Dockerfile" {
				directories = append(directories, filepath.Dir(wdFile.Name()))
			}
			continue
		}

		// Add sub-directories containing a Dockerfile one level below current directory.
		subFiles, err := afero.ReadDir(fs, wdFile.Name())
		if err != nil {
			return nil, fmt.Errorf("read directory: %w", err)
		}
		for _, f := range subFiles {
			// NOTE: ignore directories in sub-directories.
			if f.IsDir() {
				continue
			}

			if f.Name() == "Dockerfile" {
				directories = append(directories, wdFile.Name())
			}
		}
	}
	if len(directories) == 0 {
		return nil, fmt.Errorf("no Dockerfiles found within the current working directory or a sub-directory level below")
	}
	sort.Strings(directories)
	var dockerfiles []string
	for _, dir := range directories {
		file := dir + "/Dockerfile"
		dockerfiles = append(dockerfiles, file)
	}
	return dockerfiles, nil
}
