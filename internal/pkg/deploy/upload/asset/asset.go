// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package asset provides functionality to manage static assets.
package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// UploadFunc is the function signature to upload contents to a destination.
type UploadFunc func(dest string, contents io.Reader) (url string, err error)

type Uploader struct {
	FS     afero.Fs
	Upload UploadFunc
}

// UploadOpts contains optional configuration for uploading assets.
type UploadOpts struct {
	Reincludes []string // Relative path under source to reinclude files that are excluded in the upload.
	Excludes   []string // Relative path under source to exclude in the upload.
	Recursive  bool     // Whether to walk recursively.
}

// CachedAsset represents an S3 object uploaded to a temporary bucket that needs
// to be moved from a cached location to the destination bucket/key.
type CachedAsset struct {
	LocalPath      string
	RemotePath     string
	Data           io.Reader
	DestinationKey string
}

// UploadToCache ...
func (u *Uploader) UploadToCache(sourcePath, destPath string, opts *UploadOpts) ([]CachedAsset, error) {
	matcher := buildCompositeMatchers(buildReincludeMatchers(opts.Reincludes), buildExcludeMatchers(opts.Excludes))
	info, err := u.FS.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", sourcePath, err)
	}

	paths := []string{sourcePath}
	if info.IsDir() {
		files, err := afero.ReadDir(u.FS, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read directory %q: %w", sourcePath, err)
		}

		paths = make([]string, len(files))
		for i, f := range files {
			paths[i] = filepath.Join(sourcePath, f.Name())
		}
	}

	var assets []CachedAsset
	for _, path := range paths {
		if err := afero.Walk(u.FS, path, u.walkFn(sourcePath, destPath, opts.Recursive, matcher, &assets)); err != nil {
			return nil, fmt.Errorf("walk the file tree rooted at %q: %w", path, err)
		}
	}

	// upload assets

	return assets, nil
}

func (u *Uploader) walkFn(sourcePath, destPath string, recursive bool, matcher filepathMatcher, assets *[]CachedAsset) filepath.WalkFunc {
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
		file, err := u.FS.Open(path)
		if err != nil {
			return fmt.Errorf("open %q: %w", path, err)
		}
		defer file.Close()

		_, err = io.Copy(io.MultiWriter(buf, hash), file)
		if err != nil {
			return fmt.Errorf("copy %q: %w", path, err)
		}

		asset := CachedAsset{
			LocalPath:      path,
			Data:           buf,
			RemotePath:     "static-site-cache/todo-svc-name/" + hex.EncodeToString(hash.Sum(nil)),
			DestinationKey: destPath,
		}

		if sourcePath == path && destPath == "" {
			asset.DestinationKey = sourcePath
		} else if sourcePath != path {
			fileRel, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return fmt.Errorf("get relative path for %q against %q: %w", path, sourcePath, err)
			}

			asset.DestinationKey = filepath.Join(destPath, fileRel)
		}

		*assets = append(*assets, asset)
		return nil
	}
}

func (u *Uploader) uploadAssets(assets []CachedAsset) error {
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		asset := assets[i]
		g.Go(func() error {
			_, err := u.Upload(asset.RemotePath, asset.Data)
			if err != nil {
				return fmt.Errorf("upload %q: %w", asset.LocalPath, err)
			}
			return nil
		})
	}

	return g.Wait()
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
