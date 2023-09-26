// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestScaffoldWithPatch(t *testing.T) {
	t.Run("scaffolds files in an empty directory", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		dir := filepath.Join("copilot", "frontend", "overrides")

		err := ScaffoldWithPatch(fs, dir)
		require.NoError(t, err)

		ok, _ := afero.Exists(fs, filepath.Join(dir, "README.md"))
		require.True(t, ok, "README.md should exist")

		ok, _ = afero.Exists(fs, filepath.Join(dir, YAMLPatchFile))
		require.True(t, ok, "cfn.patches.yml should exist")
	})
	t.Run("should return an error if the directory is not empty", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		dir := filepath.Join("copilot", "frontend", "overrides")

		_ = fs.MkdirAll(dir, 0755)
		_ = afero.WriteFile(fs, filepath.Join(dir, "random.txt"), []byte("content"), 0644)

		err := ScaffoldWithPatch(fs, dir)
		require.EqualError(t, err, fmt.Sprintf("directory %q is not empty", dir))
	})
}

func TestPatch_Override(t *testing.T) {
	tests := map[string]struct {
		yaml        string
		overrides   string
		expected    string
		expectedErr string
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
		"add to end sequence by index": {
			yaml: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;`,
			overrides: `
- op: add
  path: /Resources/TaskDef/List/2
  value: qwerty`,
			expected: `
Resources:
  TaskDef:
    List:
      - asdf
      - jkl;
      - qwerty`,
		},
		"add to end sequence with -": {
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
		"works with special characters": {
			yaml: `
Resources:
  key:
    key~with/weirdchars/: old`,
			overrides: `
- op: replace
  path: /Resources/key/key~0with~1weirdchars~1
  value: new`,
			expected: `
Resources:
  key:
    key~with/weirdchars/: new`,
		},
		"add works with doc selector": {
			yaml: `
Resources:
  key: value`,
			overrides: `
- op: add
  path: ""
  value:
    a: aaa
    b: bbb`,
			expected: `
a: aaa
b: bbb`,
		},
		"replace works with doc selector": {
			yaml: `
Resources:
  key: value`,
			overrides: `
- op: replace
  path: ""
  value:
    a: aaa
    b: bbb`,
			expected: `
a: aaa
b: bbb`,
		},
		"remove works with doc selector": {
			yaml: `
Resources:
  key: value`,
			overrides: `
- op: remove
  path: ""`,
			expected: ``,
		},
		"empty string key works": {
			yaml: `
key: asdf
"": old`,
			overrides: `
- op: replace
  path: /
  value: new`,
			expected: `
key: asdf
"": new`,
		},
		"nothing happens with empty patch file": {
			yaml: `
a:
  b: value`,
			expected: `
a:
  b: value`,
		},
		"error on invalid patch file format": {
			overrides: `
op: add
path: /
value: new`,
			expectedErr: `file at "/cfn.patches.yml" does not conform to the YAML patch document schema: yaml: unmarshal errors:
  line 1: cannot unmarshal !!map into []override.yamlPatch`,
		},
		"error on unsupported operation": {
			overrides: `
- op: unsupported
  path: /
  value: new`,
			expectedErr: `unsupported operation "unsupported": supported operations are "add", "remove", and "replace".`,
		},
		"error in map following path": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: replace
  path: /a/e/c
  value: val`,
			expectedErr: `unable to apply the "replace" patch at index 0: key "/a": "e" not found in map`,
		},
		"error out of bounds sequence following path": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: add
  path: /a/b/3
  value: val`,
			expectedErr: `unable to apply the "add" patch at index 0: key "/a/b": index 3 out of bounds for sequence of length 2`,
		},
		"error invalid index sequence following path": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: add
  path: /a/b/e
  value: val`,
			expectedErr: `unable to apply the "add" patch at index 0: key "/a/b": expected index in sequence, got "e"`,
		},
		"error invalid index sequence - in middle of path": {
			yaml: `
a:
  b:
    - key: abcd
    - key: efgh`,
			overrides: `
- op: add
  path: /a/b/-/key
  value: val`,
			expectedErr: `unable to apply the "add" patch at index 0: key "/a/b": expected index in sequence, got "-"`,
		},
		"error targeting scalar while following path add": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: add
  path: /a/b/1/e
  value: val`,
			expectedErr: `unable to apply the "add" patch at index 0: key "/a/b/1": invalid node type scalar`,
		},
		"error targeting scalar while following path remove": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: remove
  path: /a/b/1/e`,
			expectedErr: `unable to apply the "remove" patch at index 0: key "/a/b/1": invalid node type scalar`,
		},
		"error targeting scalar while following path replace": {
			yaml: `
a:
  b:
    - c
    - d`,
			overrides: `
- op: replace
  path: /a/b/1/e
  value: val`,
			expectedErr: `unable to apply the "replace" patch at index 0: key "/a/b/1": invalid node type scalar`,
		},
		"error add with no value": {
			overrides: `
- op: add
  path: /a/b/c`,
			expectedErr: `unable to apply the "add" patch at index 0: value required`,
		},
		"error replace with no value": {
			overrides: `
- op: replace
  path: /a/b/c`,
			expectedErr: `unable to apply the "replace" patch at index 0: value required`,
		},
		"error remove nonexistant value from map": {
			yaml: `
a:
  b: value`,
			overrides: `
- op: remove
  path: /a/c`,
			expectedErr: `unable to apply the "remove" patch at index 0: key "/a": "c" not found in map`,
		},
		"error patch index incrememts": {
			yaml: `
a:
  b: value`,
			overrides: `
- op: remove
  path: /a/b
- op: remove
  path: /a/c`,
			expectedErr: `unable to apply the "remove" patch at index 1: key "/a": "c" not found in map`,
		},
		"updates the Description field of a CloudFormation template with YAML patch metrics": {
			yaml: `
Description: "CloudFormation template that represents a backend service on Amazon ECS."
Resources:
  key: value`,
			overrides: `
- op: replace
  path: /Resources/key
  value: other`,
			expected: `
Description: "CloudFormation template that represents a backend service on Amazon ECS using AWS Copilot with YAML patches."
Resources:
  key: other`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			file, err := fs.Create("/" + YAMLPatchFile)
			require.NoError(t, err)
			_, err = file.WriteString(strings.TrimSpace(tc.overrides))
			require.NoError(t, err)

			p := WithPatch("/", PatchOpts{
				FS: fs,
			})

			out, err := p.Override([]byte(strings.TrimSpace(tc.yaml)))
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)

			// convert for better comparison output
			// limitation: doesn't test for comments sticking around
			var expected interface{}
			var actual interface{}
			require.NoError(t, yaml.Unmarshal([]byte(tc.expected), &expected))
			require.NoError(t, yaml.Unmarshal(out, &actual))

			require.Equal(t, expected, actual)
		})
	}
}
