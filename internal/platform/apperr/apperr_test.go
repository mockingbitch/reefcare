package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestValidatorCollectsAllFields(t *testing.T) {
	v := NewValidator()
	if v.Err() != nil {
		t.Fatal("empty validator should return nil")
	}
	v.Check(false, "email", "required", nil)
	v.Check(true, "name", "required", nil)
	v.Check(false, "password", "too_short", map[string]string{"min": "10"})

	e, ok := As(v.Err())
	if !ok || e.Kind != KindValidation || e.Code != "validation_failed" {
		t.Fatalf("unexpected error: %#v", v.Err())
	}
	if len(e.Fields) != 2 || e.Fields[0].Field != "email" || e.Fields[1].Params["min"] != "10" {
		t.Errorf("unexpected fields: %+v", e.Fields)
	}
}

func TestWithCauseIsFoundThroughWrapping(t *testing.T) {
	cause := errors.New("db down")
	err := fmt.Errorf("service: %w", NotFound().WithCause(cause))

	if !IsKind(err, KindNotFound) {
		t.Fatal("kind not found through wrapping")
	}
	if !errors.Is(err, cause) {
		t.Fatal("cause not reachable through errors.Is")
	}
	if NotFound().Unwrap() != nil {
		t.Fatal("WithCause must not mutate the original")
	}
}
