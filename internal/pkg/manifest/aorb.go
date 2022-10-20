package manifest

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// Union is a type used for yaml keys that may be of type A or B.
// After calling yaml.Unmarshal(), you can check which type
// the yaml satisified by calling IsA() or IsB(), and get the value
// with A() or B(). Union will only ever match type A or B, never both.
//
// YAML is unmarshaled in the order A -> B. That means if both A and B are
// structs, and have common fields, A will get set and B will remain unset.
// If A is a string, B is a struct, and the yaml is an Object that doesn't match
// any keys of B, B will be set with all fields set to their zero values.
//
// TODO exported so it can be embedded. yaml tag 'inline' could help, but would require Type.Union.A on every reference
type Union[A, B any] struct {
	// IsA is true if yaml.Unmarshal successfully unmarshalled the input into A.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	isA bool

	// A holds the value of type A. If IsA is true, this representation
	// has been filled by yaml.Unmarshal. If IsA is false, or yaml.Unmarshal
	// has not been called, this is the zero value of type A.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	// TODO:
	// - exported for mergo
	A A

	// IsB is true if yaml.Unmarshal was unable to unmarshal the input into A,
	// but successfully unmarshalled the input into B.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	isB bool

	// B holds the value of type B. If IsB is true, this representation
	// has been filled by yaml.Unmarshal. If IsB is false, or yaml.Unmarshal
	// has not been called, this is the zero value of type B.
	//
	// Exported for testing, typically shouldn't be set directly outside of tests.
	B B
}

// NewUnionA is nice for tests.
func NewUnionA[A, B any](val A) Union[A, B] {
	return Union[A, B]{
		isA: true,
		A:   val,
	}
}

// NewUnionB is nice for tests.
func NewUnionB[A, B any](val B) Union[A, B] {
	return Union[A, B]{
		isB: true,
		B:   val,
	}
}

func (t *Union[_, _]) IsA() bool {
	return t.isA
}

func (t *Union[_, _]) IsB() bool {
	return t.isB
}

// TODO (t *Union) SetA()/SetB()?

// Value returns either the underlying value of A or B, depending on
// which was filled by yaml.Unmarshal. If neither have been/were set,
// Value returns nil.
func (t *Union[A, B]) Value() any {
	switch {
	case t.IsA():
		return t.A
	case t.IsB():
		return t.B
	}
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *Union[A, B]) UnmarshalYAML(value *yaml.Node) error {
	// reset struct
	var a A
	var b B
	t.isA, t.A = false, a
	t.isB, t.B = false, b

	err := value.Decode(&a)
	if err == nil {
		t.isA = true
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
		t.isB = true
		t.B = b
	}
	return err
}

// MarshalYAML implements yaml.Marshaler.
func (t Union[_, _]) MarshalYAML() (interface{}, error) {
	return t.Value(), nil
}

// IsZero implements yaml.IsZeroer.
func (t Union[_, _]) IsZero() bool {
	return !t.IsA() && !t.IsB()
}

// validate TODO
func (t Union[A, B]) validate() error {
	// type declarations inside generic functions not currently supported,
	// so we use an inline validate() interface
	if t.isA {
		if v, ok := any(t.A).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}

	if t.isB {
		if v, ok := any(t.B).(interface{ validate() error }); ok {
			return v.validate()
		}
		return nil
	}
	return nil
}
