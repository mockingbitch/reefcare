// Package logging configures the application's structured logger.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// sensitiveKeys are attribute keys whose values must never reach logs.
var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"password_hash": {},
	"token":         {},
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
	"session":       {},
	"csrf_token":    {},
	"secret":        {},
	"database_url":  {},
}

// New returns a slog.Logger writing JSON or text to w. Attributes with sensitive
// keys are redacted as a last line of defense; callers must still avoid logging secrets.
func New(w io.Writer, level slog.Level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level, ReplaceAttr: redact}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h)
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}
