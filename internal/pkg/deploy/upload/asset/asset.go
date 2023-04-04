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

// Uploader uploads local asset files.
type Uploader struct {
	// FS is the file system to use.
	FS afero.Fs

	// Upload is the function called when uploading a file.
	Upload func(bucket, key string, contents io.Reader) (string, error)

	// CachePathPrefix is the path to prefix any hashed files when uploading to the cache.
	CachePathPrefix string

	// CacheBucket is the bucket passed to Upload when uploading to the cache.
	CacheBucket string
}

// UploadOpts contains optional configuration for uploading assets.
type UploadOpts struct {
	Reincludes []string // Relative path under source to reinclude files that are excluded in the upload.
	Excludes   []string // Relative path under source to exclude in the upload.
	Recursive  bool     // Whether to walk recursively.
}

// Cached represents an S3 object uploaded to a cache bucket that needs
// to be moved from a cached location to the destination bucket/key.
type Cached struct {
	// LocalPath is the local path to the asset.
	LocalPath string

	// Content is the content of the file at LocalPath.
	Content io.Reader

	// CachePath is the uploaded location of the asset in the CacheBucket.
	CachePath string

	// CacheBucket is the bucket the file was uploaded to.
	CacheBucket string

	// DestinationPath is desired path of the file after it's copied from CacheBucket.
	DestinationPath string
}

// UploadToCache uploads the file(s) at source to u's CacheBucket.
// Returns a list of cached assets successfully uploaded to the cache and an error, if any.
func (u *Uploader) UploadToCache(source, dest string, opts *UploadOpts) ([]Cached, error) {
	matcher := buildCompositeMatchers(buildReincludeMatchers(opts.Reincludes), buildExcludeMatchers(opts.Excludes))

	var assets []Cached
	if err := afero.Walk(u.FS, source, u.walkFn(source, dest, opts.Recursive, matcher, &assets)); err != nil {
		return nil, fmt.Errorf("walk the file tree rooted at %q: %w", source, err)
	}

	if err := u.uploadAssets(assets); err != nil {
		return nil, err
	}

	return assets, nil
}

func (u *Uploader) walkFn(sourcePath, destPath string, recursive bool, matcher filepathMatcher, assets *[]Cached) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !recursive && path != sourcePath {
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

		// rel is "." when sourcePath == path
		rel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("get relative path for %q against %q: %w", path, sourcePath, err)
		}

		dest := filepath.Join(destPath, rel)
		if dest == "." { // happens when sourcePath is a file and destPath is unset
			dest = info.Name()
		}

		*assets = append(*assets, Cached{
			LocalPath:       path,
			Content:         buf,
			CachePath:       filepath.Join(u.CachePathPrefix, hex.EncodeToString(hash.Sum(nil))),
			CacheBucket:     u.CacheBucket,
			DestinationPath: dest,
		})
		return nil
	}
}

func (u *Uploader) uploadAssets(assets []Cached) error {
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		asset := assets[i]
		g.Go(func() error {
			_, err := u.Upload(asset.CacheBucket, asset.CachePath, asset.Content)
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
