// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package customresource provides functionality to upload Copilot custom resources.
package asset

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
)

type filterAction int

const (
	include filterAction = iota + 1
	exclude
)

// UploadFunc is the function signature to upload contents to a destination.
type UploadFunc func(dest string, contents io.Reader) (url string, err error)

// UploadInput is the input of Upload.
type UploadInput struct {
	Source      string
	Destination string
	Includes    []string
	Excludes    []string
	Upload      UploadFunc
	Reader      *afero.Afero
}

// Upload uploads static assets to Cloud Storage.
func Upload(in *UploadInput) ([]string, error) {
	files, err := listFiles(in.Reader, in.Source)
	if err != nil {
		return nil, err
	}
	filter := filter{buildPatterns(in.Includes, in.Excludes)}
	filteredFiles, err := filter.apply(files)
	if err != nil {
		return nil, err
	}
	// TODO: read file and upload. Remove file names from return.
	return filteredFiles, nil
}

func buildPatterns(includes, excludes []string) []pattern {
	var filterPatterns []pattern
	// Make sure exclude patterns are applied before include patterns.
	for _, syntax := range excludes {
		filterPatterns = append(filterPatterns, pattern{
			action: exclude,
			syntax: syntax,
		})
	}
	for _, syntax := range includes {
		filterPatterns = append(filterPatterns, pattern{
			action: include,
			syntax: syntax,
		})
	}
	return filterPatterns
}

func listFiles(fs *afero.Afero, path string) ([]string, error) {
	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", path, err)
	}
	var found []string
	for _, file := range files {
		if file.IsDir() {
			subFound, err := listFiles(fs, filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}
			found = append(found, subFound...)
		} else {
			found = append(found, filepath.Join(path, file.Name()))
		}
	}
	return found, nil
}

// pattern uses shell file name pattern.
type pattern struct {
	action filterAction
	syntax string
}

type filter struct {
	patterns []pattern
}

func (f *filter) apply(files []string) ([]string, error) {
	var filtered []string
	for _, file := range files {
		shouldInclude := true
		for _, p := range f.patterns {
			isMatch, err := filepath.Match(p.syntax, file)
			if err != nil {
				return nil, fmt.Errorf("match file path %s against pattern %s: %w", file, p.syntax, err)
			}
			if !isMatch {
				continue
			}
			shouldInclude = p.action == include
		}
		if shouldInclude {
			filtered = append(filtered, file)
		}
	}
	return filtered, nil
}
