package manifest

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// AOrB is a type used for yaml keys that may be of type A or B.
// After calling yaml.Unmarshal(), you can check which type
// the yaml satisified by calling IsA() or IsB(), and get the value
// with A() or B(). AOrB will only ever match type A or B, never both.
//
// YAML is unmarshaled in the order A -> B. That means if both A and B are
// structs, and have common fields, A will get set and B will remain unset.
// If A is a string, B is a struct, and the yaml is an Object that doesn't match
// any keys of B, B will be set with all fields set to their zero values.
type AOrB[A, B any] struct {
	// IsA is true if yaml.Unmarshal successfully unmarshalled the input into A.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	IsA bool

	// A holds the value of type A. If IsA is true, this representation
	// has been filled by yaml.Unmarshal. If IsA is false, or yaml.Unmarshal
	// has not been called, this is the zero value of type A.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	A A

	// IsB is true if yaml.Unmarshal was unable to unmarshal the input into A,
	// but successfully unmarshalled the input into B.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	IsB bool

	// B holds the value of type B. If IsB is true, this representation
	// has been filled by yaml.Unmarshal. If IsB is false, or yaml.Unmarshal
	// has not been called, this is the zero value of type B.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	B B
}

// Value returns either the underlying value of A or B, depending on
// which was filled by yaml.Unmarshal. If neither have been/were set,
// Value returns nil.
func (t *AOrB[A, B]) Value() any {
	switch {
	case t.IsA:
		return t.A
	case t.IsB:
		return t.B
	}
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *AOrB[A, B]) UnmarshalYAML(value *yaml.Node) error {
	// reset struct
	var a A
	var b B
	t.IsA = false
	t.A = a
	t.IsB = false
	t.B = b

	err := value.Decode(&a)
	if err == nil {
		t.IsA = true
		t.A = a
		return nil
	}

	// if the error wasn't just the inability to unmarshal
	// into A, then return that error
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}

	err = value.Decode(&b)
	if err == nil {
		t.IsB = true
		t.B = b
	}
	return err
}

// MarshalYAML implements yaml.Marshaler.
func (t AOrB[_, _]) MarshalYAML() (interface{}, error) {
	return t.Value(), nil
}

// IsZero implements yaml.IsZeroer.
func (t AOrB[_, _]) IsZero() bool {
	return !t.IsA && !t.IsB
}
