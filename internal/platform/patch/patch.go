// Package patch supports JSON merge-patch request bodies (docs/api-guidelines.md §1):
// a Field distinguishes "absent" (leave unchanged), "null" (clear) and a value.
package patch

import (
	"bytes"
	"encoding/json"
)

// Field is one optional member of a PATCH body. Use it as a non-pointer struct field.
type Field[T any] struct {
	Set   bool // the member was present in the body
	Null  bool // the member was JSON null
	Value T    // valid when Set && !Null
}

// UnmarshalJSON records presence. encoding/json calls it for null as well, and adds
// the field name to any *json.UnmarshalTypeError it returns.
func (f *Field[T]) UnmarshalJSON(b []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		f.Null = true
		var zero T
		f.Value = zero
		return nil
	}
	f.Null = false
	// Nested objects reject unknown members, like the top-level body does.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(&f.Value)
}

// Of returns a set, non-null field (for tests and internal callers).
func Of[T any](v T) Field[T] { return Field[T]{Set: true, Value: v} }

// Apply returns the patched value of a nullable attribute: unchanged when absent,
// nil when null, otherwise a pointer to the new value.
func (f Field[T]) Apply(current *T) *T {
	switch {
	case !f.Set:
		return current
	case f.Null:
		return nil
	default:
		v := f.Value
		return &v
	}
}
