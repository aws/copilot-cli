// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package asset provides functionality to manage static assets.
package asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"
)

// UploadFunc is the function signature to upload contents to a destination.
type UploadFunc func(dest string, contents io.Reader) (url string, err error)

// UploadOpts contains optional configuration for uploading assets.
type UploadOpts struct {
	Reincludes []string   // Relative path under source to reinclude files that are excluded in the upload.
	Excludes   []string   // Relative path under source to exclude in the upload.
	UploadFn   UploadFunc // Custom implementation on how to upload the contents under a file. Defaults to S3UploadFn.
	Recursive  bool       // Whether to walk recursively.
}

type Asset struct {
	LocalPath  string
	RemotePath string
	Data       io.Reader
}

// CachedAsset represents an S3 object uploaded to a temporary bucket that needs
// to be moved from a cached location to the destination bucket/key.
type CachedAsset struct {
	Asset          Asset
	DestinationKey string
}

// Upload uploads static assets to Cloud Storage and returns uploaded file URLs.
func Upload(fs afero.Fs, source, destination string, opts *UploadOpts) ([]CachedAsset, error) {
	matcher := buildCompositeMatchers(buildReincludeMatchers(opts.Reincludes), buildExcludeMatchers(opts.Excludes))
	info, err := fs.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("get stat for file %q: %w", source, err)
	}
	paths := []string{source}
	if info.IsDir() {
		files, err := afero.ReadDir(fs, source)
		if err != nil {
			return nil, fmt.Errorf("read directory %q: %w", source, err)
		}
		paths = make([]string, len(files))
		for i, f := range files {
			paths[i] = filepath.Join(source, f.Name())
		}
	} else if destination == "" { // only applies to files, not folders
		destination = source
	}

	var assets []CachedAsset
	for _, path := range paths {
		if err := afero.Walk(fs, path, walkFn(source, destination, opts.Recursive, fs, opts.UploadFn, &assets, matcher)); err != nil {
			return nil, fmt.Errorf("walk the file tree rooted at %q: %w", source, err)
		}
	}

	for _, asset := range assets {
		_, err := opts.UploadFn(asset.Asset.RemotePath, asset.Asset.Data)
		if err != nil {
			return nil, fmt.Errorf("upload file %q to destination %q: %w", asset.Asset.LocalPath, asset.Asset.RemotePath, err)
		}
		fmt.Printf("Successfully uploaded %q to %q\n", asset.Asset.LocalPath, asset.Asset.RemotePath)
	}

	return assets, nil
}

func walkFn(source, dest string, recursive bool, reader afero.Fs, upload UploadFunc, assets *[]CachedAsset, matcher filepathMatcher) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !recursive {
				return fs.SkipDir
			}
			return nil
		}
		ok, err := matcher.match(path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		hash := sha256.New()
		buf := &bytes.Buffer{}
		file, err := reader.Open(path)
		if err != nil {
			return fmt.Errorf("open file on path %q: %w", path, err)
		}
		defer file.Close()

		_, err = io.Copy(io.MultiWriter(buf, hash), file)
		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}

		fileRel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("get relative path for %q against %q: %w", path, source, err)
		}

		*assets = append(*assets, CachedAsset{
			Asset: Asset{
				LocalPath:  fileRel,
				Data:       buf,
				RemotePath: "static-site-cache/todo-svc-name/" + hex.EncodeToString(hash.Sum(nil)),
			},
			DestinationKey: filepath.Join(dest, fileRel),
		})

		return nil
	}
}

type filepathMatcher interface {
	match(path string) (bool, error)
}

type reincludeMatcher string

func buildReincludeMatchers(reincludes []string) []filepathMatcher {
	var matchers []filepathMatcher
	for _, reinclude := range reincludes {
		matchers = append(matchers, reincludeMatcher(reinclude))
	}
	return matchers
}

func (m reincludeMatcher) match(path string) (bool, error) {
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

// compositeMatcher is a composite matcher consisting of reinclude matchers and exclude matchers.
// Note that exclude matchers will be applied before reinclude matchers.
type compositeMatcher struct {
	reincludeMatchers []filepathMatcher
	excludeMatchers   []filepathMatcher
}

func buildCompositeMatchers(reincludeMatchers, excludeMatchers []filepathMatcher) compositeMatcher {
	return compositeMatcher{
		reincludeMatchers: reincludeMatchers,
		excludeMatchers:   excludeMatchers,
	}
}

func (m compositeMatcher) match(path string) (bool, error) {
	shouldInclude := true
	for _, matcher := range m.excludeMatchers {
		isMatch, err := matcher.match(path)
		if err != nil {
			return false, err
		}
		if isMatch {
			shouldInclude = false
		}
	}
	for _, matcher := range m.reincludeMatchers {
		isMatch, err := matcher.match(path)
		if err != nil {
			return false, err
		}
		if isMatch {
			shouldInclude = true
		}
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
