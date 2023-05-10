// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"sync"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	mu   sync.Mutex
	data map[string][]byte
	err  error
}

func (f *fakeS3) Upload(path string, data io.Reader) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}

	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	if f.data == nil {
		f.data = make(map[string][]byte)
	}
	f.data[path] = b
	return nil
}

func Test_UploadFiles(t *testing.T) {
	const mockMappingDir, mockPrefix = "mockMappingDir", "mockPrefix"
	const mockContent1, mockContent2, mockContent3 = "mockContent1", "mockContent2", "mockContent3"

	hash := func(content string) string {
		hash := sha256.New()
		hash.Write([]byte(content))
		return hex.EncodeToString(hash.Sum(nil))
	}

	newAsset := func(dstPath string, content string, contentType string) asset {
		return asset{
			ArtifactBucketPath: path.Join(mockPrefix, hash(content)),
			content:            bytes.NewBufferString(content),
			ServiceBucketPath:  dstPath,
			ContentType:        contentType,
		}
	}

	testCases := map[string]struct {
		files          []manifest.FileUpload
		mockS3Error    error
		mockFileSystem func(fs afero.Fs)

		expected      []asset
		expectedError error
	}{
		"error if failed to upload": {
			files: []manifest.FileUpload{
				{
					Source:    "test",
					Recursive: true,
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			mockS3Error:   errors.New("mock error"),
			expectedError: fmt.Errorf(`upload assets: upload "test/copilot/.workspace": mock error`),
		},
		"success without include and exclude": {
			// source=directory, dest unset
			files: []manifest.FileUpload{
				{
					Source:    "test",
					Recursive: true,
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []asset{
				newAsset("copilot/.workspace", mockContent1, ""),
			},
		},
		"success without recursive": {
			// source=directory, dest unset
			files: []manifest.FileUpload{
				{
					Source: "test",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "test/manifest.yaml", []byte(mockContent2), 0644)
				afero.WriteFile(fs, "test/foo", []byte(mockContent3), 0644)
			},
			expected: []asset{
				newAsset("foo", mockContent3, ""),
				newAsset("manifest.yaml", mockContent2, mime.TypeByExtension(".yaml")),
			},
		},
		"success with include only": {
			// source=directory, dest set
			files: []manifest.FileUpload{
				{
					Source:      "test",
					Destination: "ws",
					Recursive:   true,
					Reinclude: manifest.StringSliceOrString{
						StringSlice: []string{"copilot/prod/manifest.yaml"},
					},
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []asset{
				newAsset("ws/copilot/.workspace", mockContent1, ""),
			},
		},
		"success with exclude only": {
			// source=directory, dest unset
			files: []manifest.FileUpload{
				{
					Recursive: true,
					Exclude: manifest.StringSliceOrString{
						StringSlice: []string{"copilot/prod/*.yaml"},
					},
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []asset{
				newAsset("test/copilot/.workspace", mockContent1, ""),
			},
		},
		"success with both include and exclude": {
			// source=directory, dest set
			files: []manifest.FileUpload{
				{
					Destination: "files",
					Recursive:   true,
					Exclude: manifest.StringSliceOrString{
						StringSlice: []string{"copilot/prod/*.yaml"},
					},
					Reinclude: manifest.StringSliceOrString{
						StringSlice: []string{"copilot/prod/manifest.yaml"},
					},
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
				afero.WriteFile(fs, "copilot/prod/foo.yaml", []byte(mockContent3), 0644)
			},
			expected: []asset{
				newAsset("files/copilot/prod/manifest.yaml", mockContent2, mime.TypeByExtension(".yaml")),
				newAsset("files/test/copilot/.workspace", mockContent1, ""),
			},
		},
		"success with file as source": {
			// source=file, dest unset
			files: []manifest.FileUpload{
				{
					Source: "test/copilot/.workspace",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
			},
			expected: []asset{
				newAsset(".workspace", mockContent1, ""),
			},
		},
		"success with file as source and destination set": {
			// source=file, dest set
			files: []manifest.FileUpload{
				{
					Source:      "test/copilot/.workspace",
					Destination: "/is/a/file",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
			},
			expected: []asset{
				newAsset("/is/a/file", mockContent1, ""),
			},
		},
		"duplicate file mappings dedupe'd": {
			files: []manifest.FileUpload{
				{
					Source:      "dir/file.json",
					Destination: "dir/file.json",
				},
				{
					Source:      "dir/file.txt",
					Destination: "dir/file.txt",
				},
				{
					Source:      "dir",
					Destination: "dir",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "dir/file.json", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "dir/file.txt", []byte(mockContent1), 0644)
			},
			expected: []asset{
				newAsset("dir/file.json", mockContent1, mime.TypeByExtension(".json")),
				newAsset("dir/file.txt", mockContent1, mime.TypeByExtension(".txt")),
			},
		},
		"duplicate content to separate destinations sorted": {
			files: []manifest.FileUpload{
				{
					Source:      "dir/file.txt",
					Destination: "dir/file.txt",
				},
				{
					Source: "dir",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "dir/file.txt", []byte(mockContent1), 0644)
			},
			expected: []asset{
				// dir/file.txt sorts before file.txt
				newAsset("dir/file.txt", mockContent1, mime.TypeByExtension(".txt")),
				newAsset("file.txt", mockContent1, mime.TypeByExtension(".txt")),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// build the expected s3 bucket
			expected := make(map[string][]byte)
			for _, asset := range tc.expected {
				expected[asset.ArtifactBucketPath] = asset.content.(*bytes.Buffer).Bytes()
			}

			// add in the mapping file
			b, err := json.Marshal(tc.expected)
			require.NoError(t, err)

			hash := sha256.New()
			hash.Write(b)

			expectedMappingFilePath := path.Join(mockMappingDir, hex.EncodeToString(hash.Sum(nil)))
			expected[expectedMappingFilePath] = b

			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			mockS3 := &fakeS3{
				err: tc.mockS3Error,
			}

			u := ArtifactBucketUploader{
				FS:                  fs,
				Upload:              mockS3.Upload,
				AssetDir:            mockPrefix,
				AssetMappingFileDir: mockMappingDir,
			}

			mappingFilePath, err := u.UploadFiles(tc.files)
			if tc.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, expectedMappingFilePath, mappingFilePath)
			require.Equal(t, expected, mockS3.data)
		})
	}
}
