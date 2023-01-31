// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func Test_Upload(t *testing.T) {
	const mockContent = "mockContent"
	testCases := map[string]struct {
		inSource       string
		inIncludes     []string
		inExcludes     []string
		mockFileSystem func(fs afero.Fs)

		expectedFiles []string
		expectedError error
	}{
		"success without include and exclude": {
			inSource: "./test",
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			expectedFiles: []string{"test/copilot/.workspace"},
		},
		"success with include only": {
			inSource:   "./test",
			inIncludes: []string{"copilot/prod/manifest.yaml"},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			expectedFiles: []string{"test/copilot/.workspace"},
		},
		"success with exclude only": {
			inSource:   "./",
			inExcludes: []string{"copilot/prod/*.yaml"},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
			},
			expectedFiles: []string{"test/copilot/.workspace"},
		},
		"success with both include and exclude": {
			inSource:   "./",
			inExcludes: []string{"copilot/prod/*.yaml"},
			inIncludes: []string{"copilot/prod/manifest.yaml"},
			mockFileSystem: func(fs afero.Fs) {
				afero.WriteFile(fs, "test/copilot/.workspace", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/manifest.yaml", []byte(mockContent), 0644)
				afero.WriteFile(fs, "copilot/prod/foo.yaml", []byte(mockContent), 0644)
			},
			expectedFiles: []string{"test/copilot/.workspace", "copilot/prod/manifest.yaml"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Create an empty FileSystem
			fs := afero.NewMemMapFs()
			// Set it up
			tc.mockFileSystem(fs)

			files, err := Upload(&UploadInput{
				Source:   tc.inSource,
				Excludes: tc.inExcludes,
				Includes: tc.inIncludes,
				Reader:   &afero.Afero{Fs: fs},
			})
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedFiles, files)
			} else {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}
}
