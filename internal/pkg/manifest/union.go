// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Union is a type used for yaml keys that may be of type Basic or Advanced.
// Union will only ever hold one of the underlying types, never both.
//
// Union is exported to enable type embedding.
type Union[Basic, Advanced any] struct {
	// isBasic is true if the underlying type of Union is Basic.
	isBasic bool

	// Basic holds the value of Union if IsBasic() is true.
	// If IsBasic() is false, this is the zero value of type Basic.
	//
	// Basic is exported to support mergo. It should not be set
	// directly. Use NewUnionBasic() to create a Union with Basic set.
	Basic Basic

	// isAdvanced is true if the underlying type of Union is Advanced.
	isAdvanced bool

	// Advanced holds the value of Union if IsAdvanced() is true.
	// If IsAdvanced() is false, this is the zero value of type Advanced.
	//
	// Advanced is exported to support mergo. It should not be set
	// directly. Use NewUnionAdvanced() to create a Union with Advanced set.
	Advanced Advanced
}

// BasicToUnion creates a new Union[Basic, Advanced] with the underlying
// type set to Basic, holding val.
func BasicToUnion[Basic, Advanced any](val Basic) Union[Basic, Advanced] {
	return Union[Basic, Advanced]{
		isBasic: true,
		Basic:   val,
	}
}

// AdvancedToUnion creates a new Union[Basic, Advanced] with the underlying
// type set to Advanced, holding val.
func AdvancedToUnion[Basic, Advanced any](val Advanced) Union[Basic, Advanced] {
	return Union[Basic, Advanced]{
		isAdvanced: true,
		Advanced:   val,
	}
}

// IsBasic returns true if the underlying value of t is type Basic.
func (t Union[_, _]) IsBasic() bool {
	return t.isBasic
}

// IsAdvanced returns true if the underlying value of t is type Advanced.
func (t Union[_, _]) IsAdvanced() bool {
	return t.isAdvanced
}

// UnmarshalYAML decodes value into either type Basic or Advanced, and stores that value
// in t. value is first decoded into type Basic, and t will hold type Basic if
// (1) There was no error decoding value into type Basic and
// (2) Basic.IsZero() returns false OR Basic is not zero via reflection.
//
// If Basic didn't meet the above criteria, then value is decoded into type Advanced.
// t will hold type Advanced if Advanced meets the same conditions that were required for type Basic.
//
// If value fails to decode into either type, or both types are zero after
// decoding, t will not hold any value.
func (t *Union[Basic, Advanced]) UnmarshalYAML(value *yaml.Node) error {
	// convert value to []byte so we can use KnownFields()
	// see https://github.com/go-yaml/yaml/issues/460
	// side effect: TypeError line numbers are reset.
	b, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	var basic Basic
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	bErr := dec.Decode(&basic)
	if bErr == nil && !isZero(basic) {
		t.SetBasic(basic)
		return nil
	}

	var advanced Advanced
	dec = yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	aErr := dec.Decode(&advanced)
	if aErr == nil && !isZero(advanced) {
		t.SetAdvanced(advanced)
		return nil
	}

	switch {
	case bErr == nil && aErr == nil:
		return nil
	case bErr != nil && aErr == nil:
		return bErr
	case aErr != nil && bErr == nil:
		return aErr
	}

	// multiline error because yaml.TypeError (which this likely is)
	// is already a multiline error
	return fmt.Errorf("%T: %s\n%T: %s", t.Basic, bErr, t.Advanced, aErr)
}

// isZero returns true if:
//   - v is a yaml.Zeroer and IsZero().
//   - v is not a yaml.Zeroer and determined to be zero via reflection.
func isZero(v any) bool {
	if z, ok := v.(yaml.IsZeroer); ok {
		return z.IsZero()
	}
	return reflect.ValueOf(v).IsZero()
}

// MarshalYAML implements yaml.Marshaler.
func (t Union[_, _]) MarshalYAML() (interface{}, error) {
	switch {
	case t.IsBasic():
		return t.Basic, nil
	case t.IsAdvanced():
		return t.Advanced, nil
	}
	return nil, nil
}

// IsZero returns true if the set value of t
// is determined to be zero via yaml.Zeroer
// or reflection. It also returns true if
// neither value for t is set.
func (t Union[_, _]) IsZero() bool {
	if t.IsBasic() {
		return isZero(t.Basic)
	}
	if t.IsAdvanced() {
		return isZero(t.Advanced)
	}
	return true
}

// validate calls t.validate() on the set value of t. If the
// current value doesn't have a validate() function, it returns nil.
func (t Union[_, _]) validate() error {
	// type declarations inside generic functions not currently supported,
	// so we use an inline validate() interface
	if t.IsBasic() {
		if v, ok := any(t.Basic).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}

	if t.IsAdvanced() {
		if v, ok := any(t.Advanced).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}
	return nil
}

// SetBasic changes the value of the Union to v.
func (t *Union[Basic, Advanced]) SetBasic(v Basic) {
	var zero Advanced
	t.isAdvanced, t.Advanced = false, zero
	t.isBasic, t.Basic = true, v
}

// SetAdvanced changes the value of the Union to v.
func (t *Union[Basic, Advanced]) SetAdvanced(v Advanced) {
	var zero Basic
	t.isBasic, t.Basic = false, zero
	t.isAdvanced, t.Advanced = true, v
}
