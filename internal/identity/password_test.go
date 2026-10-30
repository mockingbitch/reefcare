package identity

import (
	"context"
	"strings"
	"testing"

	"reefcare/internal/platform/apperr"
)

// cheap parameters keep tests fast; production uses DefaultArgon2Params.
var testParams = Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1}

var ctx = context.Background()

func TestDeriveGivesUpWhenSlotsAreBusy(t *testing.T) {
	h := NewHasher(testParams, 1)
	h.slots <- struct{}{} // occupy the only slot
	defer func() { <-h.slots }()

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := h.Hash(cctx, "correct horse battery")
	if !apperr.IsKind(err, apperr.KindUnavailable) {
		t.Fatalf("want 503 unavailable when no slot frees up, got %v", err)
	}
}

func TestHashAndVerify(t *testing.T) {
	h := NewHasher(testParams, 0)
	encoded, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Fatalf("unexpected format: %s", encoded)
	}
	if again, _ := h.Hash(ctx, "correct horse battery"); again == encoded {
		t.Fatal("hashes must be salted")
	}

	match, rehash, err := h.Verify(ctx, "correct horse battery", encoded)
	if err != nil || !match || rehash {
		t.Fatalf("verify: match=%v rehash=%v err=%v", match, rehash, err)
	}
	if match, _, _ := h.Verify(ctx, "wrong horse battery", encoded); match {
		t.Fatal("wrong password matched")
	}
}

func TestVerifyFlagsOutdatedParams(t *testing.T) {
	old, err := NewHasher(testParams, 0).Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	upgraded := NewHasher(Argon2Params{MemoryKiB: 2048, Iterations: 2, Parallelism: 1}, 0)
	match, rehash, err := upgraded.Verify(ctx, "correct horse battery", old)
	if err != nil || !match || !rehash {
		t.Fatalf("want match with rehash, got match=%v rehash=%v err=%v", match, rehash, err)
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := NewHasher(testParams, 0)
	for _, bad := range []string{
		"", "plaintext", "$argon2i$v=19$m=1024,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=16$m=1024,t=1,p=1$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=x,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=1024,t=1,p=1$!!!$aGFzaA",
	} {
		if match, _, err := h.Verify(ctx, "whatever", bad); err == nil || match {
			t.Errorf("%q: want error, got match=%v err=%v", bad, match, err)
		}
	}
}

func TestPasswordPolicy(t *testing.T) {
	tests := []struct {
		password, email, want string
	}{
		{"short", "a@example.com", "too_short"},
		{strings.Repeat("x", 129), "a@example.com", "too_long"},
		{"Password123", "a@example.com", "too_common"},
		{"QWERTYUIOP", "a@example.com", "too_common"},
		{"lan.nguyen.reef", "lan.nguyen.reef@example.com", "matches_email"},
		{"correct horse battery", "a@example.com", ""},
		{"rạn san hô xanh", "a@example.com", ""}, // 15 runes, multi-byte
	}
	for _, tt := range tests {
		if got := passwordProblem(tt.password, tt.email); got != tt.want {
			t.Errorf("passwordProblem(%q) = %q, want %q", tt.password, got, tt.want)
		}
	}
}

func TestTokens(t *testing.T) {
	raw, encoded, err := newSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 32 || len(hashToken(encoded)) != 32 {
		t.Fatal("unexpected lengths")
	}
	if string(hashToken(encoded)) == string(hashToken(encoded+"x")) {
		t.Fatal("different tokens must hash differently")
	}
	if !validCSRF(csrfToken(raw), raw) {
		t.Fatal("csrf token must validate against its secret")
	}
	if validCSRF("", raw) || validCSRF("not base64!", raw) || validCSRF(csrfToken(make([]byte, 32)), raw) {
		t.Fatal("invalid csrf tokens must be rejected")
	}
}
