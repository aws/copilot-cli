//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestPermissions_Boundary(t *testing.T) {
	t.Run("every CloudFormation template must contain conditional permissions boundary field for all IAM roles", func(t *testing.T) {
		err := filepath.WalkDir("templates", func(path string, di fs.DirEntry, err error) error {
			if !di.IsDir() {
				contents, err := os.ReadFile(path)
				require.NoError(t, err, "read file at %s", path)
				roleCount := bytes.Count(contents, []byte("AWS::IAM::Role"))
				pbFieldCount := bytes.Count(contents, []byte("PermissionsBoundary:"))

				require.Equal(t, roleCount, pbFieldCount, "number of IAM roles does not equal number of permissions boundary fields in file '%s'", path)
			}
			return nil
		})
		require.NoError(t, err, "should walk templates dir for template files")
	})
}
