// Package idempotency reads the Idempotency-Key header that makes a create safe to retry
// (docs/api-guidelines.md §1). Modules store the key and the request hash on the created row
// with a unique (recorder, key) index: a retry returns the first result, and the same key
// with a different request is rejected.
package idempotency

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
)

// Header is the request header name.
const Header = "Idempotency-Key"

// ReplayedHeader is set on a response that returns an earlier result.
const ReplayedHeader = "Idempotent-Replayed"

// Key returns the trimmed header value, or nil when absent.
func Key(r *http.Request) *string {
	k := strings.TrimSpace(r.Header.Get(Header))
	if k == "" {
		return nil
	}
	return &k
}

// Valid accepts 8–128 printable ASCII characters.
func Valid(k string) bool {
	if len(k) < 8 || len(k) > 128 {
		return false
	}
	for i := range len(k) {
		if k[i] < 0x21 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

// Hash covers the route and the decoded body (re-encoded, so formatting differences don't
// matter) to detect a key reused for another request.
func Hash(route string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte(route+"\n"), b...))
	return sum[:], nil
}
