package patch

import (
	"encoding/json"
	"errors"
	"testing"
)

type body struct {
	Name Field[string] `json:"name"`
	Bio  Field[string] `json:"bio"`
	Age  Field[int]    `json:"age"`
}

func TestFieldPresence(t *testing.T) {
	var b body
	if err := json.Unmarshal([]byte(`{"name":"Lan","bio":null}`), &b); err != nil {
		t.Fatal(err)
	}
	if !b.Name.Set || b.Name.Null || b.Name.Value != "Lan" {
		t.Errorf("name: %+v", b.Name)
	}
	if !b.Bio.Set || !b.Bio.Null {
		t.Errorf("bio: %+v", b.Bio)
	}
	if b.Age.Set {
		t.Errorf("age must be absent: %+v", b.Age)
	}

	old := "old"
	if got := b.Bio.Apply(&old); got != nil {
		t.Errorf("null must clear, got %q", *got)
	}
	if got := b.Age.Apply(nil); got != nil {
		t.Error("absent must keep current")
	}
	if got := b.Name.Apply(&old); got == nil || *got != "Lan" {
		t.Errorf("set must replace: %v", got)
	}
}

// encoding/json drops the field name for errors from custom unmarshalers;
// httpserver.DecodeJSON restores it. The error must stay an UnmarshalTypeError of the
// inner type so the client is told the expected JSON kind.
func TestFieldWrongTypeKeepsInnerType(t *testing.T) {
	var b body
	err := json.Unmarshal([]byte(`{"age":"ten"}`), &b)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Type.Name() != "int" {
		t.Fatalf("want UnmarshalTypeError for int, got %#v", err)
	}
}
