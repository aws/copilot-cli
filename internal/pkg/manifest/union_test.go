// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type unionTest[A, B any] struct {
	yaml   string
	strict bool

	expectedValue        Union[A, B]
	expectedUnmarshalErr string
	expectedYAML         string
}

func TestUnion(t *testing.T) {
	runUnionTest(t, "string or []string, is string", unionTest[string, []string]{
		yaml:          `key: hello`,
		expectedValue: BasicToUnion[string, []string]("hello"),
	})
	runUnionTest(t, "string or []string, is empty string", unionTest[string, []string]{
		yaml:          `key: ""`,
		expectedValue: BasicToUnion[string, []string](""),
	})
	runUnionTest(t, "string or []string, is []string", unionTest[string, []string]{
		yaml: `
key:
  - asdf
  - jkl;`,
		expectedValue: NewUnionAdvanced[string]([]string{"asdf", "jkl;"}),
	})
	runUnionTest(t, "bool or semiComplexStruct, is false bool", unionTest[bool, semiComplexStruct]{
		yaml:          `key: false`,
		expectedValue: BasicToUnion[bool, semiComplexStruct](false),
	})
	runUnionTest(t, "bool or semiComplexStruct, is true bool", unionTest[bool, semiComplexStruct]{
		yaml:          `key: true`,
		expectedValue: BasicToUnion[bool, semiComplexStruct](true),
	})
	runUnionTest(t, "bool or semiComplexStruct, is semiComplexStruct with all fields set", unionTest[bool, semiComplexStruct]{
		yaml: `
key:
  str: asdf
  bool: true
  int: 420
  str_ptr: jkl;
  bool_ptr: false
  int_ptr: 70`,
		expectedValue: NewUnionAdvanced[bool](semiComplexStruct{
			Str:     "asdf",
			Bool:    true,
			Int:     420,
			StrPtr:  aws.String("jkl;"),
			BoolPtr: aws.Bool(false),
			IntPtr:  aws.Int(70),
		}),
	})
	runUnionTest(t, "bool or semiComplexStruct, is semiComplexStruct without strs set", unionTest[bool, semiComplexStruct]{
		yaml: `
key:
  bool: true
  int: 420
  bool_ptr: false
  int_ptr: 70`,
		expectedValue: NewUnionAdvanced[bool](semiComplexStruct{
			Bool:    true,
			Int:     420,
			BoolPtr: aws.Bool(false),
			IntPtr:  aws.Int(70),
		}),
	})
	runUnionTest(t, "complexStruct or semiComplexStruct, is complexStruct with all fields", unionTest[complexStruct, semiComplexStruct]{
		yaml: `
key:
  str_ptr: qwerty
  semi_complex_struct:
    str: asdf
    bool: true
    int: 420
    str_ptr: jkl;
    bool_ptr: false
    int_ptr: 70`,
		expectedValue: BasicToUnion[complexStruct, semiComplexStruct](complexStruct{
			StrPtr: aws.String("qwerty"),
			SemiComplexStruct: semiComplexStruct{
				Str:     "asdf",
				Bool:    true,
				Int:     420,
				StrPtr:  aws.String("jkl;"),
				BoolPtr: aws.Bool(false),
				IntPtr:  aws.Int(70),
			},
		}),
	})
	runUnionTest(t, "complexStruct or semiComplexStruct, defaults to complexStruct not yaml.IsZeroer", unionTest[complexStruct, semiComplexStruct]{
		yaml: `
key:
  str: asdf
  bool: true
  int: 420
  str_ptr: jkl;
  bool_ptr: false
  int_ptr: 70`,
		expectedValue: BasicToUnion[complexStruct, semiComplexStruct](complexStruct{
			StrPtr: aws.String("jkl;"),
		}),
		expectedYAML: `
key:
  str_ptr: jkl;
  semi_complex_struct:
    bool: false
    int: 0`,
	})
	runUnionTest(t, "two structs, basic type doesn't support IsZero, correct yaml", unionTest[notIsZeroer, isZeroer]{
		yaml: `
key:
  subkey: hello`,
		expectedValue: BasicToUnion[notIsZeroer, isZeroer](notIsZeroer{"hello"}),
	})
	runUnionTest(t, "two structs, basic type doesn't support IsZero, incorrect yaml", unionTest[notIsZeroer, isZeroer]{
		yaml: `
key:
  randomkey: hello`,
		expectedValue: BasicToUnion[notIsZeroer, isZeroer](notIsZeroer{}),
		expectedYAML: `
key:
  subkey: ""`,
	})
	runUnionTest(t, "two structs, basic type supports IsZero, correct yaml", unionTest[isZeroer, notIsZeroer]{
		yaml: `
key:
  subkey: hello`,
		expectedValue: BasicToUnion[isZeroer, notIsZeroer](isZeroer{"hello"}),
	})
	runUnionTest(t, "two structs, basic type supports IsZero, incorrect yaml", unionTest[isZeroer, notIsZeroer]{
		yaml: `
key:
  randomkey: hello`,
		expectedValue: NewUnionAdvanced[isZeroer](notIsZeroer{}),
		expectedYAML: `
key:
  subkey: ""`,
	})
	runUnionTest(t, "string or bool, is []string, error", unionTest[string, bool]{
		yaml: `
key:
  - asdf`,
		expectedUnmarshalErr: "yaml: unmarshal errors:\n  line 3: cannot unmarshal !!seq into string",
		expectedYAML:         `key: null`,
	})
	runUnionTest(t, "bool or string, is []string, error", unionTest[bool, string]{
		yaml: `
key:
  - asdf`,
		expectedUnmarshalErr: "yaml: unmarshal errors:\n  line 3: cannot unmarshal !!seq into bool",
		expectedYAML:         `key: null`,
	})
	runUnionTest(t, "isZeroer or int, is random object, error", unionTest[isZeroer, int]{
		yaml: `
key:
  randomkey: asdf`,
		expectedUnmarshalErr: "yaml: unmarshal errors:\n  line 3: cannot unmarshal !!map into int",
		expectedYAML:         `key: null`,
	})
	runUnionTest(t, "strict mode, string or semiComplexStruct, is semiComplexStruct with a few fields set", unionTest[string, semiComplexStruct]{
		yaml: `
key:
  bool: true
  int: 420`,
		strict: true,
		expectedValue: NewUnionAdvanced[string](semiComplexStruct{
			Bool: true,
			Int:  420,
		}),
	})
	runUnionTest(t, "strict mode, string or semiComplexStruct, is semiComplexStruct with an invalid field set, error", unionTest[string, semiComplexStruct]{
		yaml: `
key:
  bool: true
  int: 420
  bad: true`,
		// This test _should_ have an unmarshal error, but decoders created through
		// value.Decode() don't inherit the parent decoder's settings:
		// https://github.com/go-yaml/yaml/issues/460
		strict: true,
		expectedValue: NewUnionAdvanced[string](semiComplexStruct{
			Bool: true,
			Int:  420,
		}),
		expectedYAML: `
key:
  bool: true
  int: 420`,
	})
	runUnionTest(t, "[]string or semiComplexStruct, is []string", unionTest[[]string, semiComplexStruct]{
		yaml: `
key:
  - asdf`,
		expectedValue: BasicToUnion[[]string, semiComplexStruct]([]string{"asdf"}),
	})
	runUnionTest(t, "[]string or semiComplexStruct, is semiComplexStruct", unionTest[[]string, semiComplexStruct]{
		yaml: `
key:
  bool: true
  int: 420`,
		expectedValue: NewUnionAdvanced[[]string](semiComplexStruct{
			Bool: true,
			Int:  420,
		}),
	})
	runUnionTest(t, "[]string or semiComplexStruct, is string, error", unionTest[[]string, semiComplexStruct]{
		yaml:                 `key: asdf`,
		expectedUnmarshalErr: "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `asdf` into []string",
		expectedYAML:         `key: null`,
	})
	runUnionTest(t, "string or semiComplexStruct, never instantiated", unionTest[string, semiComplexStruct]{
		yaml:          `wrongkey: asdf`,
		expectedValue: Union[string, semiComplexStruct]{},
		expectedYAML:  `key: null`,
	})
}

type keyValue[Basic, Advanced any] struct {
	Key Union[Basic, Advanced] `yaml:"key"`
}

func runUnionTest[Basic, Advanced any](t *testing.T, name string, test unionTest[Basic, Advanced]) {
	t.Run(name, func(t *testing.T) {
		var kv keyValue[Basic, Advanced]
		dec := yaml.NewDecoder(strings.NewReader(test.yaml))
		dec.KnownFields(test.strict)

		err := dec.Decode(&kv)
		if test.expectedUnmarshalErr != "" {
			require.EqualError(t, err, test.expectedUnmarshalErr)
		} else {
			require.NoError(t, err)
		}

		require.Equal(t, test.expectedValue, kv.Key)

		// call Marshal() with an indent of 2 spaces
		buf := &bytes.Buffer{}
		enc := yaml.NewEncoder(buf)
		enc.SetIndent(2)
		err = enc.Encode(kv)
		require.NoError(t, err)
		require.NoError(t, enc.Close())

		expectedYAML := test.yaml
		if test.expectedYAML != "" {
			expectedYAML = test.expectedYAML
		}

		// verify the marshaled string matches the input string
		require.Equal(t, strings.TrimSpace(expectedYAML), strings.TrimSpace(buf.String()))
	})
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
		Key: embeddedType{NewUnionAdvanced[string]([]string{
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
		Key: embeddedType{BasicToUnion[string, []string]("qwerty")},
	}, kv)
}

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

type notIsZeroer struct {
	SubKey string `yaml:"subkey"`
}

type isZeroer struct {
	SubKey string `yaml:"subkey"`
}

func (a isZeroer) IsZero() bool {
	return a.SubKey == ""
}
