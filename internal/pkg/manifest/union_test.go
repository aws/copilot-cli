package manifest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type yamlTestStruct[A, B any] struct {
	Key Union[A, B] `yaml:"key,omitempty"`
}

func (k *yamlTestStruct[A, B]) KeyValue() any {
	return k.Key.Value()
}

func TestAOrB(t *testing.T) {
	type semiComplexStruct struct {
		Str     string  `yaml:"str,omitempty"`
		Bool    bool    `yaml:"bool"`
		Int     int     `yaml:"int"`
		StrPtr  *string `yaml:"str_ptr,omitempty"`
		BoolPtr *bool   `yaml:"bool_ptr,omitempty"`
		IntPtr  *int    `yaml:"int_ptr,omitempty"`
	}
	type complexStruct struct {
		StrPtr            *string           `yaml:"str_ptr,omitempty"`
		SemiComplexStruct semiComplexStruct `yaml:"semi_complex_struct"`
	}

	tests := map[string]struct {
		yaml string
		val  interface {
			KeyValue() any
		}
		expectedValue        any
		expectedUnmarshalErr string

		expectedYAML *string
	}{
		"string or string slice, is string": {
			yaml: `
key: value
`,
			val:           &yamlTestStruct[string, []string]{},
			expectedValue: "value",
		},
		"string or string slice, is empty string": {
			yaml: `
key: ""
`,
			val:           &yamlTestStruct[string, []string]{},
			expectedValue: "",
		},
		"string or string slice, is string slice": {
			yaml: `
key:
  - asdf
  - jkl;
`,
			val:           &yamlTestStruct[string, []string]{},
			expectedValue: []string{"asdf", "jkl;"},
		},
		"bool or semiComplexStruct, is false bool": {
			yaml: `
key: false
`,
			val:           &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: false,
		},
		"bool or semiComplexStruct, is true bool": {
			yaml: `
key: true
`,
			val:           &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: true,
		},
		"bool or semiComplexStruct, is semiCompexStruct with all fields set": {
			yaml: `
key:
  str: asdf
  bool: true
  int: 420
  str_ptr: jkl;
  bool_ptr: false
  int_ptr: 70
`,
			val: &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: semiComplexStruct{
				Str:     "asdf",
				Bool:    true,
				Int:     420,
				StrPtr:  aws.String("jkl;"),
				BoolPtr: aws.Bool(false),
				IntPtr:  aws.Int(70),
			},
		},
		"bool or semiComplexStruct, is semiCompexStruct without strs set": {
			yaml: `
key:
  bool: true
  int: 420
  bool_ptr: false
  int_ptr: 70
`,
			val: &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: semiComplexStruct{
				Bool:    true,
				Int:     420,
				BoolPtr: aws.Bool(false),
				IntPtr:  aws.Int(70),
			},
		},
		"bool or semiComplexStruct, is semiCompexStruct without bool_ptr set": {
			yaml: `
key:
  str: asdf
  bool: false
  int: 420
  str_ptr: jkl;
  int_ptr: 70
`,
			val: &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: semiComplexStruct{
				Str:    "asdf",
				Bool:   false,
				Int:    420,
				StrPtr: aws.String("jkl;"),
				IntPtr: aws.Int(70),
			},
		},
		"bool or semiComplexStruct, is semiCompexStruct without int_ptr set": {
			yaml: `
key:
  str: asdf
  bool: true
  int: 0
  str_ptr: jkl;
  bool_ptr: false
`,
			val: &yamlTestStruct[bool, semiComplexStruct]{},
			expectedValue: semiComplexStruct{
				Str:     "asdf",
				Bool:    true,
				Int:     0,
				StrPtr:  aws.String("jkl;"),
				BoolPtr: aws.Bool(false),
			},
		},
		"complexStruct or semiComplexStruct, is complexStruct with all fields": {
			yaml: `
key:
  str_ptr: qwerty
  semi_complex_struct:
    str: asdf
    bool: true
    int: 420
    str_ptr: jkl;
    bool_ptr: false
    int_ptr: 70
`,
			val: &yamlTestStruct[complexStruct, semiComplexStruct]{},
			expectedValue: complexStruct{
				StrPtr: aws.String("qwerty"),
				SemiComplexStruct: semiComplexStruct{
					Str:     "asdf",
					Bool:    true,
					Int:     420,
					StrPtr:  aws.String("jkl;"),
					BoolPtr: aws.Bool(false),
					IntPtr:  aws.Int(70),
				},
			},
		},
		"complexStruct or semiComplexStruct, defaults to complexStruct": {
			yaml: `
key:
  str: asdf
  bool: true
  int: 420
  str_ptr: jkl;
  bool_ptr: false
  int_ptr: 70
`,
			val: &yamlTestStruct[complexStruct, semiComplexStruct]{},
			expectedValue: complexStruct{
				StrPtr: aws.String("jkl;"),
			},
			expectedYAML: aws.String(`
key:
  str_ptr: jkl;
  semi_complex_struct:
    bool: false
    int: 0
`),
		},
		"string or semiComplexStruct, is string slice, marshal IsZero": {
			yaml: `
key:
  - asdf
`,
			val:                  &yamlTestStruct[string, semiComplexStruct]{},
			expectedUnmarshalErr: "yaml: unmarshal errors:\n  line 3: cannot unmarshal !!seq into manifest.semiComplexStruct",
			expectedValue:        nil,
			expectedYAML:         aws.String("{}"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// unmarshal tc.yaml into tc.val
			err := yaml.Unmarshal([]byte(tc.yaml), tc.val)
			if tc.expectedUnmarshalErr != "" {
				require.EqualError(t, err, tc.expectedUnmarshalErr)
			} else {
				require.NoError(t, err)
			}

			// make sure the value of `key:` matches tc.expectedValue
			require.Equal(t, tc.expectedValue, tc.val.KeyValue())

			// call Marshal() with an indent of 2 spaces
			buf := &bytes.Buffer{}
			enc := yaml.NewEncoder(buf)
			enc.SetIndent(2)
			err = enc.Encode(tc.val)
			require.NoError(t, err)
			require.NoError(t, enc.Close())

			expectedYAML := tc.yaml
			if tc.expectedYAML != nil {
				expectedYAML = *tc.expectedYAML
			}

			// verify the marshaled string matches the input string
			require.Equal(t, strings.TrimSpace(expectedYAML), strings.TrimSpace(buf.String()))
		})
	}
}

func TestUnion_EmbeddedType(t *testing.T) {
	type embeddedType struct {
		Union[string, []string]
	}

	type keyValue struct {
		Key embeddedType `yaml:"key,omitempty"`
	}

	// test []string
	in := `
key:
  - asdf
`
	var kv keyValue
	require.NoError(t, yaml.Unmarshal([]byte(in), &kv))
	require.Equal(t, keyValue{
		Key: embeddedType{NewUnionB[string]([]string{
			"asdf",
		})},
	}, kv)

	// test string
	in = `
key: qwerty
`
	kv = keyValue{}
	require.NoError(t, yaml.Unmarshal([]byte(in), &kv))
	require.Equal(t, keyValue{
		Key: embeddedType{NewUnionA[string, []string]("querty")},
	}, kv)
}
