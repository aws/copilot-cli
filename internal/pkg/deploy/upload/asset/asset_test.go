// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	mu   sync.Mutex
	data map[string]string
	err  error
}

func (f *fakeS3) Upload(path string, data io.Reader) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}

	b, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}

	if f.data == nil {
		f.data = make(map[string]string)
	}
	f.data[path] = string(b)
	return nil
}

func Test_UploadFiles(t *testing.T) {
	const mockMappingPath, mockPrefix = "mockMappingPath", "mockPrefix"
	const mockContent1, mockContent2, mockContent3 = "mockContent1", "mockContent2", "mockContent3"

	cachePath := func(content string) string {
		hash := sha256.New()
		hash.Write([]byte(content))
		return filepath.Join(mockPrefix, hex.EncodeToString(hash.Sum(nil)))
	}

	mappingFile := func(assets []asset) string {
		b, err := json.Marshal(assets)
		require.NoError(t, err)
		return string(b)
	}

	testCases := map[string]struct {
		files          []manifest.FileUpload
		mockS3Error    error
		mockFileSystem func(fs afero.Fs)

		expected      map[string]string
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
			expectedError: fmt.Errorf(`upload "test/copilot/.workspace": mock error`),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent1),
						DestinationPath: "copilot/.workspace",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent2): mockContent2,
				cachePath(mockContent3): mockContent3,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent3),
						DestinationPath: "foo",
					},
					{
						Path:            cachePath(mockContent2),
						DestinationPath: "manifest.yaml",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent1),
						DestinationPath: "ws/copilot/.workspace",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent1),
						DestinationPath: "test/copilot/.workspace",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				cachePath(mockContent2): mockContent2,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent2),
						DestinationPath: "files/copilot/prod/manifest.yaml",
					},
					{
						Path:            cachePath(mockContent1),
						DestinationPath: "files/test/copilot/.workspace",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent1),
						DestinationPath: ".workspace",
					},
				}),
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
			expected: map[string]string{
				cachePath(mockContent1): mockContent1,
				mockMappingPath: mappingFile([]asset{
					{
						Path:            cachePath(mockContent1),
						DestinationPath: "/is/a/file",
					},
				}),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			mockS3 := &fakeS3{
				err: tc.mockS3Error,
			}

			u := ArtifactBucketUploader{
				FS:               fs,
				Upload:           mockS3.Upload,
				PathPrefix:       mockPrefix,
				AssetMappingPath: mockMappingPath,
			}

			err := u.UploadFiles(tc.files)
			if tc.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, mockS3.data)
		})
	}
}
