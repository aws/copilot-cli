package template

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPermissions_Boundary(t *testing.T) {
	t.Run("CloudFormation template must contain conditional permissions boundary field for all IAM roles", func(t *testing.T) {
		cmd := exec.Command("find", "templates", "-name", "*yml")
		output, err := cmd.Output()
		require.NoError(t, err, "should return output of 'find' command")
		
		files := strings.Fields(string(output))
		var totalIAMRoles int
		var totalPermissionsBoundaryFields int
		for _, file := range files {
			contents, err := os.ReadFile(file)
			require.NoError(t, err, "should read file %s", file)
			IAMRoles := bytes.Count(contents, []byte("AWS::IAM::Role"))
			permissionsBoundaryFields := bytes.Count(contents, []byte("PermissionsBoundary:"))
				
			totalIAMRoles = totalIAMRoles + IAMRoles
			totalPermissionsBoundaryFields = totalPermissionsBoundaryFields + permissionsBoundaryFields
		}
		require.True(t, totalIAMRoles == totalPermissionsBoundaryFields, "every IAM role in templates should have a conditional permissions boundary field")
	})
}
