// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestDockerignoreFile(t *testing.T) {
	var (
		defaultPath = "./"
	)

	testCases := map[string]struct {
		dockerignoreFilePath string
		dockerignoreFile     []byte
		wantedExcludes       []string
		wantedErr            error
	}{
		"reads file that doesn't exist": {
			dockerignoreFilePath: "./falsepath",
			wantedExcludes:       nil,
		},
		"dockerignore file is empty": {
			dockerignoreFilePath: defaultPath,
			wantedExcludes:       nil,
		},
		"parse dockerignore file": {
			dockerignoreFilePath: defaultPath,
			dockerignoreFile: []byte(`
copilot/*
copilot/*/*
# commenteddir/*
/copilot/*
    testdir/
			`),
			wantedExcludes: []string{
				"copilot/*",
				"copilot/*/*",
				"copilot/*",
				"testdir",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./.dockerignore", tc.dockerignoreFile, 0644)
			if err != nil {
				t.FailNow()
			}

			actualExcludes, err := ReadDockerignore(fs.Fs, tc.dockerignoreFilePath)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedExcludes, actualExcludes)
			}
		})
	}
}
