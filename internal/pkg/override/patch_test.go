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
  path: /Resources/TaskDef
  value:
    Properties:
      Prop1: value
      Prop2: false`,
			expected: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
    Properties:
      Prop1: value
      Prop2: false`,
		},
		"add to sequence": {
			yaml: `
Resources:
  IAMRole:
    Type: AWS::IAM::Role
    Properties:
      Policies:
        - PolicyName: "Test"
          PolicyDocument:
            Version: '2012-10-17'
            Statement:
              - Effect: Allow
                Action:
                  - "*"
                  - "**"
                Resource:
                  - "*"`,
			overrides: `
- op: add
  path: /Resources/IAMRole/Properties/Policies/0/PolicyDocument/Statement/0/Action/-
  value:
    key: value
    key2: value2`,
			expected: `
Resources:
  IAMRole:
    Type: AWS::IAM::Role
    Properties:
      Policies:
        - PolicyName: "Test"
          PolicyDocument:
            Version: '2012-10-17'
            Statement:
              - Effect: Allow
                Action:
                  - "*"
                  - "**"
                  - key: value
                    key2: value2
                Resource:
                  - "*"`,
		},
		"remove from map": {
			yaml: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
    Description: asdf`,
			overrides: `
- op: remove
  path: /Resources/TaskDef/Description`,
			expected: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition`,
		},
		"replace scalar with scalar": {
			yaml: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
    Description: asdf`,
			overrides: `
- op: replace
  path: /Resources/TaskDef/Description
  value: jkl;`,
			expected: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
    Description: jkl;`,
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
