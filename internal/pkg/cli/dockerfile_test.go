// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestValidateDockerfiles(t *testing.T) {
	wantedDockerfiles := []string{"./Dockerfile", "backend/Dockerfile", "frontend/Dockerfile"}
	testCases := map[string]struct {
		mockFileSystem func(mockFS afero.Fs)
		err            error
	}{
		"find Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			err: nil,
		},
		"no Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {},
			err:            fmt.Errorf("no Dockerfiles found within . or a sub-directory level below"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			got, err := listDockerfiles(fs, ".")

			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, wantedDockerfiles, got)
			}
		})
	}
}
