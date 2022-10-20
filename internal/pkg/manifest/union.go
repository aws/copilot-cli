package manifest

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// Union is a type used for yaml keys that may be of type A or B.
// Union will only ever hold one of the underlying types, never both.
//
// Use NewUnionA() and NewUnionB() to create a Union with an underlying
// type already set. See Unmarshal() for details on how yaml is decoded
// into Union.
//
// Union is exported to enable type embedding.
type Union[A, B any] struct {
	// isA is true if the underlying type of Union is A.
	isA bool

	// A holds the value of Union if IsA() is true.
	// If IsA() is false, this is the zero value of type A.
	//
	// A is exported to support mergo. It should not be set
	// directly. Use NewUnionA() to create a Union with A set.
	A A

	// isB is true if the underlying type of Union is B.
	isB bool

	// B holds the value of Union if IsB() is true.
	// If IsB() is false, this is the zero value of type B.
	//
	// B is exported to support mergo. It should not be set
	// directly. Use NewUnionB() to create a Union with B set.
	B B
}

// NewUnionA creates a new Union[A, B] with the underlying
// type set to A, holding val.
func NewUnionA[A, B any](val A) Union[A, B] {
	return Union[A, B]{
		isA: true,
		A:   val,
	}
}

// NewUnionB creates a new Union[A, B] with the underlying
// type set to B, holding val.
func NewUnionB[A, B any](val B) Union[A, B] {
	return Union[A, B]{
		isB: true,
		B:   val,
	}
}

// IsA returns true if the underlying value of t is type A.
func (t *Union[_, _]) IsA() bool {
	return t.isA
}

// IsB returns true if the underlying value of t is type B.
func (t *Union[_, _]) IsB() bool {
	return t.isB
}

// Value returns either the underlying value of t, which is either type
// A or B, depending on which was filled by yaml.Unmarshal or created
// with NewUnionA/B(). If neither have been/were set, Value returns nil.
func (t *Union[A, B]) Value() any {
	switch {
	case t.IsA():
		return t.A
	case t.IsB():
		return t.B
	}
	return nil
}

// UnmarshalYAML decodes value into either type A or B, and stores that value
// on t. value is first decoded into type A, and t will hold type A if:
//   - there was no error decoding value into type A
//   - A.IsZero() returns false OR A does not support IsZero()
//
// If there was an error or A IsZero, then value is decoded into type B.
// t will hold type B if B meets the same conditions that were required for type A.
//
// If value fails to decode into either type, or both types are zero after
// decoding, t will not hold any value.
func (t *Union[A, B]) UnmarshalYAML(value *yaml.Node) error {
	// reset struct
	var a A
	var b B
	t.isA, t.A = false, a
	t.isB, t.B = false, b

	err := value.Decode(&a)
	if err == nil && !isZeroerAndZero(a) {
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
	if err == nil && !isZeroerAndZero(b) {
		t.isB = true
		t.B = b
	}
	return err
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
	return t.Value(), nil
}

// IsZero returns true if the set value of t implements
// yaml.IsZeroer and IsZero. It also returns true if
// neither value for t is set.
func (t Union[_, _]) IsZero() bool {
	if t.IsA() {
		return isZeroerAndZero(t.A)
	}
	if t.IsB() {
		return isZeroerAndZero(t.B)
	}
	return true
}

// validate calls t.validate() on the set value of t. If the
// current value doesn't have a validate() function, it returns nil.
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
