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
	"sort"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// ArtifactBucketUploader uploads local asset files.
type ArtifactBucketUploader struct {
	// FS is the file system to use.
	FS afero.Fs

	// Upload is the function called when uploading a file.
	Upload func(path string, contents io.Reader) error

	// PathPrefix is the path to prefix any hashed files when uploading to the artifact bucket.
	PathPrefix string

	// AssetMappingPath is the path the upload the asset mapping file to.
	AssetMappingPath string
}

type asset struct {
	localPath string
	content   io.Reader

	Path            string `json:"path"`
	DestinationPath string `json:"destPath"`
}

// UploadFiles hashes each of the files specified in files and uploads
// them to the path "{CachePathPrefix}/{hash}". After, it uploads a JSON file
// to CacheMovePath that specifies the uploaded location of every file and it's
// intended destination path.
func (u *ArtifactBucketUploader) UploadFiles(files []manifest.FileUpload) error {
	var assets []asset
	for _, f := range files {
		matcher := buildCompositeMatchers(buildReincludeMatchers(f.Reinclude.ToStringSlice()), buildExcludeMatchers(f.Exclude.ToStringSlice()))
		source := filepath.Join(f.Context, f.Source)

		if err := afero.Walk(u.FS, source, u.walkFn(source, f.Destination, f.Recursive, matcher, &assets)); err != nil {
			return fmt.Errorf("walk the file tree rooted at %q: %s", source, err)
		}
	}

	if err := u.uploadAssets(assets); err != nil {
		return fmt.Errorf("upload assets: %s", err)
	}

	if err := u.uploadAssetMappingFile(assets); err != nil {
		return fmt.Errorf("upload asset mapping file: %s", err)
	}
	return nil
}

func (u *ArtifactBucketUploader) walkFn(sourcePath, destPath string, recursive bool, matcher filepathMatcher, assets *[]asset) filepath.WalkFunc {
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

		*assets = append(*assets, asset{
			localPath:       path,
			content:         buf,
			Path:            filepath.Join(u.PathPrefix, hex.EncodeToString(hash.Sum(nil))),
			DestinationPath: dest,
		})
		return nil
	}
}

func (u *ArtifactBucketUploader) uploadAssets(assets []asset) error {
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		asset := assets[i]
		g.Go(func() error {
			if err := u.Upload(asset.Path, asset.content); err != nil {
				return fmt.Errorf("upload %q: %w", asset.localPath, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (u *ArtifactBucketUploader) uploadAssetMappingFile(assets []asset) error {
	// stable output
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Path != assets[j].Path {
			return assets[i].Path < assets[j].Path
		}
		return assets[i].DestinationPath < assets[j].DestinationPath
	})

	data, err := json.Marshal(assets)
	if err != nil {
		return fmt.Errorf("encode uploaded assets: %w", err)
	}

	if err := u.Upload(u.AssetMappingPath, bytes.NewBuffer(data)); err != nil {
		return fmt.Errorf("upload to %q: %w", u.AssetMappingPath, err)
	}
	return nil
}
