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
	"mime"
	"path"
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

	// AssetDir is the directory to upload the hashed files to.
	AssetDir string

	// AssetMappingFileDir is the directory to upload the asset mapping file to.
	AssetMappingFileDir string
}

type asset struct {
	localPath string
	content   io.Reader

	ArtifactBucketPath string `json:"path"`
	ServiceBucketPath  string `json:"destPath"`
	ContentType        string `json:"contentType"`
}

// UploadFiles hashes each of the files specified in files and uploads
// them to the path "{AssetDir}/{hash}". After, it uploads a JSON file
// to AssetDir that maps the location of every file in the artifact bucket to its
// intended destination path in the service bucket. The path to the mapping file
// is returned along with an error, if any.
func (u *ArtifactBucketUploader) UploadFiles(files []manifest.FileUpload) (string, error) {
	var assets []asset
	for _, f := range files {
		matcher := buildCompositeMatchers(buildReincludeMatchers(f.Reinclude.ToStringSlice()), buildExcludeMatchers(f.Exclude.ToStringSlice()))

		if err := afero.Walk(u.FS, f.Source, u.walkFn(f.Source, f.Destination, f.Recursive, matcher, &assets)); err != nil {
			return "", fmt.Errorf("walk the file tree rooted at %q: %s", f.Source, err)
		}
	}

	if err := u.uploadAssets(assets); err != nil {
		return "", fmt.Errorf("upload assets: %s", err)
	}

	path, err := u.uploadAssetMappingFile(assets)
	if err != nil {
		return "", fmt.Errorf("upload asset mapping file: %s", err)
	}
	return path, nil
}

func (u *ArtifactBucketUploader) walkFn(sourcePath, destPath string, recursive bool, matcher filepathMatcher, assets *[]asset) filepath.WalkFunc {
	return func(fpath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// if path == sourcePath, then path is the directory they want uploaded.
			// if path != sourcePath, then path is a _subdirectory_ of the directory they want uploaded.
			if !recursive && fpath != sourcePath {
				return fs.SkipDir
			}
			return nil
		}
		ok, err := matcher.match(fpath)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		hash := sha256.New()
		buf := &bytes.Buffer{}
		file, err := u.FS.Open(fpath)
		if err != nil {
			return fmt.Errorf("open %q: %w", fpath, err)
		}
		defer file.Close()

		_, err = io.Copy(io.MultiWriter(buf, hash), file)
		if err != nil {
			return fmt.Errorf("copy %q: %w", fpath, err)
		}

		// rel is "." when sourcePath == path
		rel, err := filepath.Rel(sourcePath, fpath)
		if err != nil {
			return fmt.Errorf("get relative path for %q against %q: %w", fpath, sourcePath, err)
		}

		dest := filepath.Join(destPath, rel)
		if dest == "." { // happens when sourcePath is a file and destPath is unset
			dest = info.Name()
		}

		*assets = append(*assets, asset{
			localPath:          fpath,
			content:            buf,
			ArtifactBucketPath: path.Join(u.AssetDir, hex.EncodeToString(hash.Sum(nil))),
			ServiceBucketPath:  filepath.ToSlash(dest),
			ContentType:        mime.TypeByExtension(filepath.Ext(fpath)),
		})
		return nil
	}
}

func (u *ArtifactBucketUploader) uploadAssets(assets []asset) error {
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		asset := assets[i]
		g.Go(func() error {
			if err := u.Upload(asset.ArtifactBucketPath, asset.content); err != nil {
				return fmt.Errorf("upload %q: %w", asset.localPath, err)
			}
			return nil
		})
	}

	return g.Wait()
}

// uploadAssetMappingFile uploads a JSON file containing the location
// of each file in the artifact bucket and the desired location
// of the file in the destination bucket. It has the format:
//
//	[{
//	  "path": "local-assets/12345asdf",
//	  "destPath": "index.html",
//	  "contentType": "text/html"
//	}]
//
// The path returned is u.AssetMappingDir/a hash of the mapping file's content.
// This makes it so the file path is constant as long as the
// content and destination of the uploaded assets do not change.
func (u *ArtifactBucketUploader) uploadAssetMappingFile(assets []asset) (string, error) {
	assets = dedupe(assets)
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].ArtifactBucketPath != assets[j].ArtifactBucketPath {
			return assets[i].ArtifactBucketPath < assets[j].ArtifactBucketPath
		}
		return assets[i].ServiceBucketPath < assets[j].ServiceBucketPath
	})

	data, err := json.Marshal(assets)
	if err != nil {
		return "", fmt.Errorf("encode uploaded assets: %w", err)
	}

	hash := sha256.New()
	hash.Write(data) // hash.Write is documented to never return an error

	uploadedPath := path.Join(u.AssetMappingFileDir, hex.EncodeToString(hash.Sum(nil)))
	if err := u.Upload(uploadedPath, bytes.NewBuffer(data)); err != nil {
		return "", fmt.Errorf("upload to %q: %w", u.AssetMappingFileDir, err)
	}
	return uploadedPath, nil
}

// dedupe returns a copy of assets with duplicate entries removed.
func dedupe(assets []asset) []asset {
	type key struct{ field1, field2, field3 string }
	has := make(map[key]bool)
	out := make([]asset, 0, len(assets))

	for i := range assets {
		key := key{assets[i].ArtifactBucketPath, assets[i].ServiceBucketPath, assets[i].ContentType}
		if has[key] {
			continue
		}
		has[key] = true
		out = append(out, assets[i])
	}

	return out
}
