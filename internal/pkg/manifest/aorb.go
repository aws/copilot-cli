package manifest

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// aOrB is a type used for yaml keys that may be of type A or B.
// After calling yaml.Unmarshal(), you can check which type
// the yaml satisified by calling IsA() or IsB(), and get the value
// with A() or B(). aOrB will only ever match type A or B, never both.
//
// YAML is unmarshaled in the order A -> B. That means if both A and B are
// structs, and have common fields, A will get set and B will remain unset.
// If A is a string, B is a struct, and the yaml is an Object that doesn't match
// any keys of B, B will be set with all fields set to their zero values.
type aOrB[A, B any] struct {
	isA bool
	a   A

	isB bool
	b   B
}

// IsA returns true if, yaml.Unmarshal successfully unmarshalled the input
// into type A.
func (t *aOrB[A, B]) IsA() bool {
	return t.isA
}

// IsA returns true if, yaml.Unmarshal was unable to unmarshal the input
// into type B, but successfully unmarshalled into type B.
func (t *aOrB[A, B]) IsB() bool {
	return t.isB
}

// A returns the underlying value of A. If IsA() returns true, this is
// the representation filled by yaml.Unmarshal. If IsA() returns false,
// or yaml.Unmarshal has not been called, this is the zero value of A.
func (t *aOrB[A, B]) A() A {
	return t.a
}

// B returns the underlying value of B. If IsB() returns true, this is
// the representation filled by yaml.Unmarshal. If IsB() returns false,
// or yaml.Unmarshal has not been called, this is the zero value of B.
func (t *aOrB[A, B]) B() B {
	return t.b
}

// Value returns either the underlying value of A or B, depending on
// which was filled by yaml.Unmarshal. If neither have been/were set,
// Value returns nil.
func (t *aOrB[A, B]) Value() any {
	switch {
	case t.IsA():
		return t.A()
	case t.IsB():
		return t.B()
	}
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *aOrB[A, B]) UnmarshalYAML(value *yaml.Node) error {
	// reset struct
	var a A
	var b B
	t.isA = false
	t.a = a
	t.isB = false
	t.b = b

	err := value.Decode(&a)
	if err == nil {
		t.isA = true
		t.a = a
		return nil
	}

	// if the error wasn't just the inability to unmarshal
	// into a, then return that error
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}

	err = value.Decode(&b)
	if err == nil {
		t.isB = true
		t.b = b
	}
	return err
}

// MarshalYAML implements yaml.Marshaler.
func (t aOrB[_, _]) MarshalYAML() (interface{}, error) {
	return t.Value(), nil
}

// IsZero implements yaml.IsZeroer.
func (t aOrB[_, _]) IsZero() bool {
	return !t.IsA() && !t.IsB()
}
