// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package asset provides functionality to manage static assets.
package asset

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"
)

// UploadFunc is the function signature to upload contents to a destination.
type UploadFunc func(dest string, contents io.Reader) (url string, err error)

type UploadOpts struct {
	Includes []string   // Relative path under source to include back files that are excluded in the upload.
	Excludes []string   // Relative path under source to exclude in the upload.
	UploadFn UploadFunc // Custom implementation on how to upload the contents under a file. Defaults to S3UploadFn.
}

// Upload uploads static assets to Cloud Storage.
func Upload(fs *afero.Afero, source, destination string, opts *UploadOpts) ([]string, error) {
	matcher := buildCompositeMatchers(buildIncludeMatchers(opts.Includes), buildExcludeMatchers(opts.Excludes))
	var paths []string
	pathsPtr := &paths
	if err := fs.Walk(source, walkFnWithMatcher(pathsPtr, matcher)); err != nil {
		return nil, fmt.Errorf("walk the file tree rooted at %s: %w", source, err)
	}
	// TODO: read file and upload. Remove file names from return.
	return *pathsPtr, nil
}

func walkFnWithMatcher(pathsPtr *[]string, matcher filepathMatcher) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ok, err := matcher.match(path)
		if err != nil {
			return err
		}
		if ok {
			*pathsPtr = append(*pathsPtr, path)
		}
		return nil
	}
}

type filepathMatcher interface {
	match(path string) (bool, error)
}

type includeMatcher string

func buildIncludeMatchers(includes []string) []filepathMatcher {
	var matchers []filepathMatcher
	for _, include := range includes {
		matchers = append(matchers, includeMatcher(include))
	}
	return matchers
}

func (m includeMatcher) match(path string) (bool, error) {
	return match(string(m), path)
}

type excludeMatcher string

func buildExcludeMatchers(excludes []string) []filepathMatcher {
	var matchers []filepathMatcher
	for _, exclude := range excludes {
		matchers = append(matchers, excludeMatcher(exclude))
	}
	return matchers
}

func (m excludeMatcher) match(path string) (bool, error) {
	return match(string(m), path)
}

// compositeMatcher is a composite matcher consisting of include matchers and exclude matchers.
// Note that exclude matchers will be applied before include matchers.
type compositeMatcher []filepathMatcher

func buildCompositeMatchers(includeMatchers, excludeMatchers []filepathMatcher) compositeMatcher {
	return append(excludeMatchers, includeMatchers...)
}

func (m compositeMatcher) match(path string) (bool, error) {
	shouldInclude := true
	for _, matcher := range m {
		isMatch, err := matcher.match(path)
		if err != nil {
			return false, err
		}
		if !isMatch {
			continue
		}
		_, shouldInclude = matcher.(includeMatcher)
	}
	return shouldInclude, nil
}

func match(pattern, path string) (bool, error) {
	isMatch, err := filepath.Match(pattern, path)
	if err != nil {
		return false, fmt.Errorf("match file path %s against pattern %s: %w", path, pattern, err)
	}
	return isMatch, nil
}
