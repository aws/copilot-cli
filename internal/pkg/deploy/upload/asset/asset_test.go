// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	err error
}

func (f *fakeS3) Upload(bucket, key string, data io.Reader) (string, error) {
	return "", f.err
}

func Test_UploadToCache(t *testing.T) {
	const mockContent1 = "mockContent1"
	const mockContent2 = "mockContent2"
	const mockContent3 = "mockContent3"
	cachePath := func(content string) string {
		hash := sha256.New()
		hash.Write([]byte(content))
		return filepath.Join("mockPrefix", hex.EncodeToString(hash.Sum(nil)))
	}

	cached := func(localPath, destPath string, content string) Cached {
		return Cached{
			LocalPath:       localPath,
			Data:            bytes.NewBuffer([]byte(content)),
			CacheBucket:     "mockBucket",
			CachePath:       cachePath(content),
			DestinationPath: destPath,
		}
	}

	testCases := map[string]struct {
		inSource       string
		inDest         string
		inReincludes   []string
		inExcludes     []string
		inRecursive    bool
		inMockS3       fakeS3
		mockFileSystem func(fs afero.Fs)

		expected      []Cached
		expectedError error
	}{
		"error if failed to upload": {
			inSource:    "test",
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			inMockS3: fakeS3{
				err: errors.New("mock error"),
			},
			expectedError: fmt.Errorf(`upload "test/copilot/.workspace": mock error`),
		},
		"success without include and exclude": {
			inSource:    "test",
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []Cached{
				cached("test/copilot/.workspace", "copilot/.workspace", mockContent1),
			},
		},
		"success without recursive": {
			inSource: "test",
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "test/manifest.yaml", []byte(mockContent2), 0644)
				afero.WriteFile(fs, "test/foo", []byte(mockContent3), 0644)
			},
			expected: []Cached{
				cached("test/foo", "foo", mockContent3),
				cached("test/manifest.yaml", "manifest.yaml", mockContent2),
			},
		},
		"success with include only": {
			inSource:     "test",
			inDest:       "ws",
			inRecursive:  true,
			inReincludes: []string{"copilot/prod/manifest.yaml"},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []Cached{
				cached("test/copilot/.workspace", "ws/copilot/.workspace", mockContent1),
			},
		},
		"success with exclude only": {
			inExcludes:  []string{"copilot/prod/*.yaml"},
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
			},
			expected: []Cached{
				cached("test/copilot/.workspace", "test/copilot/.workspace", mockContent1),
			},
		},
		"success with both include and exclude": {
			inDest:       "files",
			inExcludes:   []string{"copilot/prod/*.yaml"},
			inReincludes: []string{"copilot/prod/manifest.yaml"},
			inRecursive:  true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent2), 0644)
				afero.WriteFile(fs, "copilot/prod/foo.yaml", []byte(mockContent3), 0644)
			},
			expected: []Cached{
				cached("copilot/prod/manifest.yaml", "files/copilot/prod/manifest.yaml", mockContent2),
				cached("test/copilot/.workspace", "files/test/copilot/.workspace", mockContent1),
			},
		},
		"success with file as source": {
			inSource: "test/copilot/.workspace",
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent1), 0644)
			},
			expected: []Cached{
				cached("test/copilot/.workspace", ".workspace", mockContent1),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			u := Uploader{
				FS:              fs,
				Upload:          tc.inMockS3.Upload,
				CachePathPrefix: "mockPrefix",
				CacheBucket:     "mockBucket",
			}

			files, err := u.UploadToCache(tc.inSource, tc.inDest, &UploadOpts{
				Excludes:   tc.inExcludes,
				Reincludes: tc.inReincludes,
				Recursive:  tc.inRecursive,
			})
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expected, files)
			} else {
				require.Error(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}
