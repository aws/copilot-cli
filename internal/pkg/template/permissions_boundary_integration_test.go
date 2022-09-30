//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestPermissions_Boundary(t *testing.T) {
	t.Run("every CloudFormation template must contain conditional permissions boundary field for all IAM roles", func(t *testing.T) {
		err := filepath.WalkDir("templates", func(path string, di fs.DirEntry, err error) error {
			contents, _ := os.ReadFile(path)
			IAMRoles := bytes.Count(contents, []byte("AWS::IAM::Role"))
			permissionsBoundaryFields := bytes.Count(contents, []byte("PermissionsBoundary:"))

			msg := fmt.Sprintf("number of IAM roles (%d) does not equal number of permissions boundary fields (%d) in file '%s'", IAMRoles, permissionsBoundaryFields, path)
			require.True(t, IAMRoles == permissionsBoundaryFields, msg)
			return nil
		})
		require.NoError(t, err, "should walk templates dir for template files")
	})
}
