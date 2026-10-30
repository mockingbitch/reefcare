package idempotency

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	for k, want := range map[string]bool{
		"12345678": true, strings.Repeat("a", 128): true, "1234567": false, strings.Repeat("a", 129): false,
		"has space": false, "tab\tkey1": false, "ünïcode-key": false,
	} {
		if Valid(k) != want {
			t.Errorf("Valid(%q) != %v", k, want)
		}
	}
}

func TestKeyAndHash(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	if Key(r) != nil {
		t.Fatal("no header")
	}
	r.Header.Set(Header, "  abcdefgh  ")
	if k := Key(r); k == nil || *k != "abcdefgh" {
		t.Fatalf("key %v", k)
	}
	a, _ := Hash("POST /a", map[string]int{"x": 1})
	b, _ := Hash("POST /a", map[string]int{"x": 1})
	c, _ := Hash("POST /b", map[string]int{"x": 1})
	d, _ := Hash("POST /a", map[string]int{"x": 2})
	if !bytes.Equal(a, b) || bytes.Equal(a, c) || bytes.Equal(a, d) || len(a) != 32 {
		t.Fatal("hash must depend on route and body only")
	}
}
