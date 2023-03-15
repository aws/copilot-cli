package override

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPatch(t *testing.T) {
	tests := map[string]struct {
		yaml      string
		overrides string
		expected  string
	}{
		"add to map": {
			yaml: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition`,
			overrides: `
- op: add
  path: /Resources
  value:
    TestResource:
      Type: AWS::Danny`,
			expected: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
  TestResource:
    Type: AWS::Danny`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			file, err := fs.Create("/patches.yml")
			require.NoError(t, err)
			_, err = file.WriteString(strings.TrimSpace(tc.overrides))
			require.NoError(t, err)

			p := WithPatch("/", PatchOpts{
				FS: fs,
			})

			out, err := p.Override([]byte(strings.TrimSpace(tc.yaml)))
			require.NoError(t, err)

			// convert for better comparison
			// limitation: doesn't test for comments sticking around
			var expected map[string]interface{}
			var actual map[string]interface{}
			require.NoError(t, yaml.Unmarshal([]byte(tc.expected), &expected))
			require.NoError(t, yaml.Unmarshal(out, &actual))

			require.Equal(t, expected, actual)
		})
	}
}
