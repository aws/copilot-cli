// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	objects map[string]string
	err     error
}

func (f *fakeS3) UploadFunc() func(string, io.Reader) (string, error) {
	return func(key string, dat io.Reader) (url string, err error) {
		if f.err != nil {
			return "", f.err
		}
		url, ok := f.objects[key]
		if !ok {
			return "", fmt.Errorf("key %q does not exist in fakeS3", key)
		}
		return url, nil
	}
}

func Test_Upload(t *testing.T) {
	const mockContent = "mockContent"
	testCases := map[string]struct {
		inSource       string
		inDest         string
		inReincludes   []string
		inExcludes     []string
		inRecursive    bool
		inMockS3       fakeS3
		mockFileSystem func(fs afero.Fs)

		expectedURLs  []string
		expectedError error
	}{
		"error if failed to upload": {
			inSource:    "test",
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			inMockS3: fakeS3{
				err: errors.New("some error"),
			},
			expectedError: fmt.Errorf(`walk the file tree rooted at "test": upload file "test/copilot/.workspace" to destination "copilot/.workspace": some error`),
		},
		"success without include and exclude": {
			inSource:    "test",
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			inMockS3: fakeS3{
				objects: map[string]string{
					"copilot/.workspace": "url",
				},
			},
			expectedURLs: []string{"url"},
		},
		"success without recursive": {
			inSource: "test",
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "test/manifest.yaml", []byte(mockContent), 0644)
				afero.WriteFile(fs, "test/foo", []byte(mockContent), 0644)
			},
			inMockS3: fakeS3{
				objects: map[string]string{
					"copilot/.workspace": "url1",
					"manifest.yaml":      "url2",
					"foo":                "url3",
				},
			},
			expectedURLs: []string{"url2", "url3"},
		},
		"success with include only": {
			inSource:     "test",
			inDest:       "ws",
			inRecursive:  true,
			inReincludes: []string{"copilot/prod/manifest.yaml"},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			inMockS3: fakeS3{
				objects: map[string]string{
					"ws/copilot/.workspace": "url",
				},
			},
			expectedURLs: []string{"url"},
		},
		"success with exclude only": {
			inExcludes:  []string{"copilot/prod/*.yaml"},
			inRecursive: true,
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			inMockS3: fakeS3{
				objects: map[string]string{
					"test/copilot/.workspace": "url",
				},
			},
			expectedURLs: []string{"url"},
		},
		"success with both include and exclude": {
			inDest:       "files",
			inExcludes:   []string{"copilot/prod/*.yaml"},
			inReincludes: []string{"copilot/prod/manifest.yaml"},
			inRecursive:  true,
			inMockS3: fakeS3{
				objects: map[string]string{
					"files/test/copilot/.workspace":    "url1",
					"files/copilot/prod/manifest.yaml": "url2",
				},
			},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/foo.yaml", []byte(mockContent), 0644)
			},
			expectedURLs: []string{"url1", "url2"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			files, err := Upload(&afero.Afero{Fs: fs}, tc.inSource, tc.inDest, &UploadOpts{
				Excludes:   tc.inExcludes,
				Reincludes: tc.inReincludes,
				Recursive:  tc.inRecursive,
				UploadFn:   tc.inMockS3.UploadFunc(),
			})
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedURLs, files)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}
