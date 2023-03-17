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
		"add to map in pointer": {
			yaml: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition`,
			overrides: `
- op: add
  path: /Resources/TaskDef/Properties
  value:
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
		"add to beginning sequence": {
			yaml: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;`,
			overrides: `
- op: add
  path: /Resources/TaskDef/List/0
  value: qwerty`,
			expected: `
Resources:
  TaskDef:
    List:
      - qwerty
      - asdf
      - jkl;`,
		},
		"add to middle sequence": {
			yaml: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;`,
			overrides: `
- op: add
  path: /Resources/TaskDef/List/1
  value: qwerty`,
			expected: `
Resources:
  TaskDef:
    List:
      - asdf
      - qwerty
      - jkl;`,
		},
		"add to end sequence using index": {
			yaml: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;`,
			overrides: `
- op: add
  path: /Resources/TaskDef/List/-
  value: qwerty`,
			expected: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;
      - qwerty`,
		},
		"add to end sequence without index": {
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
  path: /Resources/IAMRole/Properties/Policies/0/PolicyDocument/Statement/0/Action
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
		"remove scalar from map": {
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
		"remove map from map": {
			yaml: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition
    Properties:
      Prop1: value
      Prop2: value`,
			overrides: `
- op: remove
  path: /Resources/TaskDef/Properties`,
			expected: `
Resources:
  TaskDef:
    Type: AWS::ECS::TaskDefinition`,
		},
		"remove from beginning of sequence": {
			yaml: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item0
      - item1`,
			overrides: `
- op: remove
  path: /Resources/1/list/0`,
			expected: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item1`,
		},
		"remove from middle of sequence": {
			yaml: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item0
      - item1
      - item2`,
			overrides: `
- op: remove
  path: /Resources/1/list/1`,
			expected: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item0
      - item2`,
		},
		"remove from end of sequence": {
			yaml: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item0
      - item1
      - item2`,
			overrides: `
- op: remove
  path: /Resources/1/list/2`,
			expected: `
Resources:
  - obj1: value
    list:
      - item0
      - item1
  - obj2: value
    list:
      - item0
      - item1`,
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
		"replace map with scalar": {
			yaml: `
Resources:
  List:
    - asdf
    - key: value
      key2: value2
    - - very list
      - many item`,
			overrides: `
- op: replace
  path: /Resources/List/1
  value: jkl;`,
			expected: `
Resources:
  List:
    - asdf
    - jkl;
    - - very list
      - many item`,
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
