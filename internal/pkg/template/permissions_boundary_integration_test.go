//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPermissions_Boundary(t *testing.T) {
	t.Run("every CloudFormation template must contain conditional permissions boundary field for all IAM roles", func(t *testing.T) {
		cmd := exec.Command("find", "templates", "-name", "*yml")
		output, err := cmd.Output()
		require.NoError(t, err, "should return output of 'find' command")
		
		files := strings.Fields(string(output))
		for _, file := range files {
			contents, err := os.ReadFile(file)
			require.NoError(t, err, "should read file")
			IAMRoles := bytes.Count(contents, []byte("AWS::IAM::Role"))
			permissionsBoundaryFields := bytes.Count(contents, []byte("PermissionsBoundary:"))
			
			msg := fmt.Sprintf("number of IAM roles (%d) does not equal number of permissions boundary fields (%d) in file '%s'", IAMRoles, permissionsBoundaryFields, file)
			require.True(t, IAMRoles == permissionsBoundaryFields, msg)

		}
	})
}
