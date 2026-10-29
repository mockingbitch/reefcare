package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"reefcare/internal/platform/apperr"
)

// Page limits (docs/api-guidelines.md §1).
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// Page is the collection response shape.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// PageParams are the parsed ?limit= and ?cursor= query parameters. Cursor is decoded
// into the keyset type K chosen by the endpoint (e.g. {OccurredAt time.Time; ID uuid.UUID}).
type PageParams[K any] struct {
	Limit  int // page size returned to the client
	Cursor *K
}

// FetchLimit is the SQL LIMIT to query with: one extra row tells NewPage whether
// another page exists. Passing Limit instead would never produce a next cursor.
func (p PageParams[K]) FetchLimit() int32 { return int32(p.Limit + 1) } //nolint:gosec // Limit ≤ MaxPageLimit

// ParsePageParams validates limit and decodes the opaque cursor.
func ParsePageParams[K any](r *http.Request) (PageParams[K], error) {
	p := PageParams[K]{Limit: DefaultPageLimit}
	q := r.URL.Query()

	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxPageLimit {
			return p, apperr.BadRequest("invalid_query", "Invalid pagination parameter.",
				apperr.FieldError{Field: "limit", Code: "out_of_range", Params: map[string]string{"min": "1", "max": strconv.Itoa(MaxPageLimit)}})
		}
		p.Limit = n
	}
	if s := q.Get("cursor"); s != "" {
		var k *K
		if err := DecodeCursor(s, &k); err != nil || k == nil {
			return p, apperr.BadRequest("invalid_query", "Invalid pagination parameter.",
				apperr.FieldError{Field: "cursor", Code: "invalid"})
		}
		p.Cursor = k
	}
	return p, nil
}

// EncodeCursor returns an opaque cursor for a keyset value.
func EncodeCursor(key any) (string, error) {
	b, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeCursor decodes a cursor produced by EncodeCursor into dst.
func DecodeCursor(s string, dst any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// NewPage builds a Page from rows fetched with LIMIT FetchLimit(): when an extra row is
// present it is dropped and its predecessor's key becomes the next cursor.
func NewPage[T any](rows []T, limit int, keyOf func(T) any) (Page[T], error) {
	page := Page[T]{Items: rows}
	if page.Items == nil {
		page.Items = []T{}
	}
	if len(rows) <= limit {
		return page, nil
	}
	page.Items = rows[:limit]
	c, err := EncodeCursor(keyOf(rows[limit-1]))
	if err != nil {
		return page, err
	}
	page.NextCursor = &c
	return page, nil
}
