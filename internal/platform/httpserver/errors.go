package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorDetail describes one invalid field or sub-problem.
type ErrorDetail struct {
	Field  string            `json:"field,omitempty"`
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}

// ErrorBody is the error envelope defined in docs/api-guidelines.md §2.
type ErrorBody struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Details   []ErrorDetail `json:"details,omitempty"`
	RequestID string        `json:"request_id,omitempty"`
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "encode response", "err", err)
	}
}

// WriteError writes the standard error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...ErrorDetail) {
	WriteJSON(w, r, status, map[string]ErrorBody{"error": {
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: RequestIDFrom(r.Context()),
	}})
}
