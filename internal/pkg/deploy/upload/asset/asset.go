// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package asset provides functionality to manage static assets.
package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// CacheUploader uploads local asset files.
type CacheUploader struct {
	// FS is the file system to use.
	FS afero.Fs

	// Upload is the function called when uploading a file.
	Upload func(path string, contents io.Reader) (string, error)

	// PathPrefix is the path to prefix any hashed files when uploading to the cache.
	PathPrefix string

	// AssetMappingPath is the path the upload the asset mapping file to.
	AssetMappingPath string
}

type cached struct {
	localPath string
	content   io.Reader

	Path            string `json:"path"`
	DestinationPath string `json:"dest_path"`
}

// UploadFiles hashes each of the files specified in files and uploads
// them to the path "{CachePathPrefix}/{hash}". After, it uploads a JSON file
// to CacheMovePath that specifies the uploaded location of every file and it's
// intended destination path.
func (u *CacheUploader) UploadFiles(files []manifest.FileUpload) (string, error) {
	var assets []cached
	for _, f := range files {
		matcher := buildCompositeMatchers(buildReincludeMatchers(f.Reinclude.ToStringSlice()), buildExcludeMatchers(f.Exclude.ToStringSlice()))
		source := filepath.Join(f.Context, f.Source)

		if err := afero.Walk(u.FS, source, u.walkFn(source, f.Destination, f.Recursive, matcher, &assets)); err != nil {
			return "", fmt.Errorf("walk the file tree rooted at %q: %w", source, err)
		}
	}

	if err := u.uploadAssets(assets); err != nil {
		return "", err
	}

	mappingFile := &bytes.Buffer{}
	if err := json.NewEncoder(mappingFile).Encode(assets); err != nil {
		return "", fmt.Errorf("unable to encode move json")
	}

	// upload move file
	url, err := u.Upload(u.AssetMappingPath, mappingFile)
	if err != nil {
		return "", fmt.Errorf("unable to upload cache move: %w", err)
	}

	return url, nil
}

func (u *CacheUploader) walkFn(sourcePath, destPath string, recursive bool, matcher filepathMatcher, assets *[]cached) filepath.WalkFunc {
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

		*assets = append(*assets, cached{
			localPath:       path,
			content:         buf,
			Path:            filepath.Join(u.PathPrefix, hex.EncodeToString(hash.Sum(nil))),
			DestinationPath: dest,
		})
		return nil
	}
}

func (u *CacheUploader) uploadAssets(assets []cached) error {
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		asset := assets[i]
		g.Go(func() error {
			_, err := u.Upload(asset.Path, asset.content)
			if err != nil {
				return fmt.Errorf("upload %q: %w", asset.localPath, err)
			}
			return nil
		})
	}

	return g.Wait()
}
