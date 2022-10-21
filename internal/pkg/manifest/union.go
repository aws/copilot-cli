// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"gopkg.in/yaml.v3"
)

// Union is a type used for yaml keys that may be of type Simple or Advanced.
// Union will only ever hold one of the underlying types, never both.
//
// Use NewUnionSimple() and NewUnionAdvanced() to create a Union with an underlying
// type already set. See Unmarshal() for details on how yaml is decoded
// into Union.
//
// Union is exported to enable type embedding.
type Union[Simple, Advanced any] struct {
	// isSimple is true if the underlying type of Union is Simple.
	isSimple bool

	// Simple holds the value of Union if IsSimple() is true.
	// If IsSimple() is false, this is the zero value of type Simple.
	//
	// Simple is exported to support mergo. It should not be set
	// directly. Use NewUnionSimple() to create a Union with Simple set.
	Simple Simple

	// isAdvanced is true if the underlying type of Union is Advanced.
	isAdvanced bool

	// Advanced holds the value of Union if IsAdvanced() is true.
	// If IsAdvanced() is false, this is the zero value of type Advanced.
	//
	// Advanced is exported to support mergo. It should not be set
	// directly. Use NewUnionAdvanced() to create a Union with Advanced set.
	Advanced Advanced
}

// NewUnionSimple creates a new Union[Simple, Advanced] with the underlying
// type set to Simple, holding val.
func NewUnionSimple[Simple, Advanced any](val Simple) Union[Simple, Advanced] {
	return Union[Simple, Advanced]{
		isSimple: true,
		Simple:   val,
	}
}

// NewUnionAdvanced creates a new Union[Simple, Advanced] with the underlying
// type set to Advanced, holding val.
func NewUnionAdvanced[Simple, Advanced any](val Advanced) Union[Simple, Advanced] {
	return Union[Simple, Advanced]{
		isAdvanced: true,
		Advanced:   val,
	}
}

// IsSimple returns true if the underlying value of t is type Simple.
func (t *Union[_, _]) IsSimple() bool {
	return t.isSimple
}

// IsAdvanced returns true if the underlying value of t is type Advanced.
func (t *Union[_, _]) IsAdvanced() bool {
	return t.isAdvanced
}

// UnmarshalYAML decodes value into either type Simple or Advanced, and stores that value
// in t. value is first decoded into type Simple, and t will hold type Simple if:
//   - There was no error decoding value into type Simple
//   - Simple.IsZero() returns false OR Simple does not support IsZero()
//
// If Simple didn't meet the above criteria, then value is decoded into type Advanced.
// t will hold type Advanced if Advanced meets the same conditions that were required for type Simple.
//
// If value fails to decode into either type, or both types are zero after
// decoding, t will not hold any value.
func (t *Union[Simple, Advanced]) UnmarshalYAML(value *yaml.Node) error {
	// reset struct
	var simple Simple
	var advanced Advanced
	t.isSimple, t.Simple = false, simple
	t.isAdvanced, t.Advanced = false, advanced

	sErr := value.Decode(&simple)
	if sErr == nil && !isZeroerAndZero(simple) {
		t.isSimple, t.Simple = true, simple
		return nil
	}

	aErr := value.Decode(&advanced)
	if aErr == nil && !isZeroerAndZero(advanced) {
		t.isAdvanced, t.Advanced = true, advanced
		return nil
	}

	if sErr != nil {
		return sErr
	}
	return aErr
}

// isZeroerAndZero returns true if v is a yaml.Zeroer
// _and_ is zero, as determined by v.IsZero().
func isZeroerAndZero(v any) bool {
	z, ok := v.(yaml.IsZeroer)
	if !ok {
		return false
	}
	return z.IsZero()
}

// MarshalYAML implements yaml.Marshaler.
func (t Union[_, _]) MarshalYAML() (interface{}, error) {
	switch {
	case t.IsSimple():
		return t.Simple, nil
	case t.IsAdvanced():
		return t.Advanced, nil
	}
	return nil, nil
}

// IsZero returns true if the set value of t implements
// yaml.IsZeroer and IsZero. It also returns true if
// neither value for t is set.
func (t Union[_, _]) IsZero() bool {
	if t.IsSimple() {
		return isZeroerAndZero(t.Simple)
	}
	if t.IsAdvanced() {
		return isZeroerAndZero(t.Advanced)
	}
	return true
}

// validate calls t.validate() on the set value of t. If the
// current value doesn't have a validate() function, it returns nil.
func (t Union[_, _]) validate() error {
	// type declarations inside generic functions not currently supported,
	// so we use an inline validate() interface
	if t.isSimple {
		if v, ok := any(t.Simple).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}

	if t.isAdvanced {
		if v, ok := any(t.Advanced).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}
	return nil
}
