package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"reefcare/internal/platform/apperr"
)

// DecodeJSON strictly decodes a single JSON object from the request body into dst.
// It rejects non-JSON content types, unknown fields, trailing data and oversized
// bodies (the LimitBody middleware sets the cap).
func DecodeJSON(r *http.Request, dst any) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return &apperr.Error{Kind: apperr.KindUnsupportedMedia, Code: "unsupported_media_type", Message: "Content-Type must be application/json."}
	}

	// The body is buffered (LimitBody caps it) so a failing custom-typed field can be
	// located afterwards; see locateField.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return decodeError(err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(locateField(body, dst, err))
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.BadRequest("malformed_json", "Request body must contain a single JSON object.")
	}
	return nil
}

// locateField adds the field name to err when encoding/json could not: errors returned
// by custom UnmarshalJSON methods (patch.Field, UUIDs, …) carry no field context, and
// unknown members of nested objects are reported without their parent. Each top-level
// member is decoded on its own into the matching struct field; the first one that fails
// names the field. Other errors are returned unchanged.
func locateField(body []byte, dst any, err error) error {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	unknown, isUnknown := strings.CutPrefix(err.Error(), "json: unknown field ")
	switch {
	case errors.As(err, &typeErr) && typeErr.Field != "",
		errors.As(err, &syntaxErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return err
	}
	t := reflect.TypeOf(dst)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return err
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(body, &members) != nil {
		return err
	}
	if isUnknown {
		if _, topLevel := members[strings.Trim(unknown, `"`)]; topLevel {
			return err
		}
	}
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		raw, ok := member(members, name)
		if !ok {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		fieldErr := dec.Decode(reflect.New(f.Type).Interface())
		if fieldErr == nil {
			continue
		}
		if child, ok := strings.CutPrefix(fieldErr.Error(), "json: unknown field "); ok {
			return fmt.Errorf("json: unknown field %q", name+"."+strings.Trim(child, `"`))
		}
		if errors.As(fieldErr, &typeErr) {
			located := *typeErr
			located.Field = name
			if typeErr.Field != "" {
				located.Field = name + "." + typeErr.Field
			}
			return &located
		}
		return &fieldError{field: name, err: fieldErr}
	}
	return err
}

// member finds a JSON member the way encoding/json matches struct fields: exact name
// first, then case-insensitively.
func member(members map[string]json.RawMessage, name string) (json.RawMessage, bool) {
	if raw, ok := members[name]; ok {
		return raw, true
	}
	for k, raw := range members {
		if strings.EqualFold(k, name) {
			return raw, true
		}
	}
	return nil, false
}

// fieldError is a value a custom unmarshaler rejected (e.g. a malformed UUID).
type fieldError struct {
	field string
	err   error
}

func (e *fieldError) Error() string { return e.field + ": " + e.err.Error() }

// jsonKind names the expected JSON type for a Go type, so clients never see Go type names.
func jsonKind(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

func decodeError(err error) error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		maxErr    *http.MaxBytesError
		fieldErr  *fieldError
	)
	switch {
	case errors.As(err, &maxErr):
		return &apperr.Error{Kind: apperr.KindTooLarge, Code: "body_too_large",
			Message: fmt.Sprintf("Request body must not exceed %d bytes.", maxErr.Limit)}
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return apperr.BadRequest("malformed_json", "Request body is not valid JSON.")
	case errors.Is(err, io.EOF):
		return apperr.BadRequest("malformed_json", "Request body must not be empty.")
	case errors.As(err, &typeErr):
		return apperr.BadRequest("malformed_json", "A field has the wrong type.",
			apperr.FieldError{Field: typeErr.Field, Code: "wrong_type", Params: map[string]string{"expected": jsonKind(typeErr.Type)}})
	case errors.As(err, &fieldErr):
		return apperr.BadRequest("malformed_json", "A field has an invalid value.",
			apperr.FieldError{Field: fieldErr.field, Code: "invalid"})
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json exposes no typed error for unknown fields.
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return apperr.BadRequest("unknown_field", "Request contains an unknown field.",
			apperr.FieldError{Field: field, Code: "unknown_field"})
	default:
		return apperr.BadRequest("malformed_json", "Request body could not be decoded.")
	}
}

var kindStatus = map[apperr.Kind]int{
	apperr.KindValidation:       http.StatusUnprocessableEntity,
	apperr.KindNotFound:         http.StatusNotFound,
	apperr.KindConflict:         http.StatusConflict,
	apperr.KindUnauthenticated:  http.StatusUnauthorized,
	apperr.KindForbidden:        http.StatusForbidden,
	apperr.KindRateLimited:      http.StatusTooManyRequests,
	apperr.KindBadRequest:       http.StatusBadRequest,
	apperr.KindTooLarge:         http.StatusRequestEntityTooLarge,
	apperr.KindUnsupportedMedia: http.StatusUnsupportedMediaType,
	apperr.KindUnavailable:      http.StatusServiceUnavailable,
}

// WriteAppError maps err to the error envelope. Application errors keep their code
// and message; anything else becomes a 500 whose details are only logged.
func WriteAppError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	e, ok := apperr.As(err)
	status, known := 0, false
	if ok {
		status, known = kindStatus[e.Kind]
	}
	if !known {
		log.ErrorContext(r.Context(), "request failed", "request_id", RequestIDFrom(r.Context()), "err", err)
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "An unexpected error occurred.")
		return
	}
	if e.Unwrap() != nil {
		log.WarnContext(r.Context(), "request rejected", "request_id", RequestIDFrom(r.Context()), "code", e.Code, "err", err)
	}
	if e.Kind == apperr.KindRateLimited || (e.Kind == apperr.KindUnavailable && e.RetryAfter > 0) {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(e.RetryAfter.Seconds())))))
	}
	details := make([]ErrorDetail, 0, len(e.Fields))
	for _, f := range e.Fields {
		details = append(details, ErrorDetail(f))
	}
	WriteError(w, r, status, e.Code, e.Message, details...)
}
