package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"reefcare/internal/platform/apperr"
)

// Argon2Params are the Argon2id cost parameters (docs/security.md §2).
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
}

// DefaultArgon2Params is the production setting: 64 MiB, 3 passes, 2 lanes.
var DefaultArgon2Params = Argon2Params{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2}

const (
	saltLen = 16
	keyLen  = 32
	// slotWait bounds how long a request queues for a hashing slot before giving up
	// with 503, so a flood cannot build an ever-growing backlog.
	slotWait = 3 * time.Second
)

var errMalformedHash = errors.New("malformed password hash")

// Hasher creates and verifies Argon2id hashes in PHC string format, e.g.
// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>. Parameters are encoded in each hash,
// so they can be raised later and old hashes upgraded on the next successful login.
type Hasher struct {
	params    Argon2Params
	dummyHash string        // verified against when the account doesn't exist, to equalize timing
	slots     chan struct{} // bounds concurrent derivations: each holds MemoryKiB of RAM
}

// NewHasher returns a Hasher using p for new hashes, running at most maxConcurrent
// derivations at once (≤ 0 means runtime.GOMAXPROCS, which respects container CPU
// limits). Size it so maxConcurrent × MemoryKiB fits the memory limit with headroom.
func NewHasher(p Argon2Params, maxConcurrent int) *Hasher {
	if maxConcurrent <= 0 {
		maxConcurrent = max(2, runtime.GOMAXPROCS(0))
	}
	h := &Hasher{params: p, slots: make(chan struct{}, maxConcurrent)}
	dummy, err := h.Hash(context.Background(), "timing-equalizer-not-a-real-password")
	if err != nil {
		panic(err) // only fails if crypto/rand fails
	}
	h.dummyHash = dummy
	return h
}

// Hash derives a new PHC-formatted Argon2id hash with a random salt.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	p := h.params
	key, err := h.derive(ctx, []byte(password), salt, p, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches encoded, and whether the hash should be
// upgraded because it was produced with parameters other than the current ones.
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (match, needsRehash bool, err error) {
	p, salt, want, err := parseHash(encoded)
	if err != nil {
		return false, false, err
	}
	got, err := h.derive(ctx, []byte(password), salt, p, uint32(len(want))) //nolint:gosec // len(want) ≤ 64, checked in parseHash
	if err != nil {
		return false, false, err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, p != h.params, nil
}

// VerifyDummy spends the same work as a real verification so unknown accounts can't be
// distinguished by response time.
func (h *Hasher) VerifyDummy(ctx context.Context, password string) error {
	_, _, err := h.Verify(ctx, password, h.dummyHash)
	return err
}

func (h *Hasher) derive(ctx context.Context, password, salt []byte, p Argon2Params, n uint32) ([]byte, error) {
	wait, cancel := context.WithTimeout(ctx, slotWait)
	defer cancel()
	select {
	case h.slots <- struct{}{}:
	case <-wait.Done():
		return nil, apperr.Unavailable(time.Second)
	}
	defer func() { <-h.slots }()
	return argon2.IDKey(password, salt, p.Iterations, p.MemoryKiB, p.Parallelism, n), nil
}

func parseHash(encoded string) (Argon2Params, []byte, []byte, error) {
	var p Argon2Params
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, errMalformedHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return p, nil, nil, errMalformedHash
	}
	return p, salt, key, nil
}

// Password policy (docs/security.md §2): length only, plus a common-password blocklist.
const (
	minPasswordLen = 10
	maxPasswordLen = 128
)

//go:embed common_passwords.txt
var commonPasswordsTxt string

var commonPasswords = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, line := range strings.Split(commonPasswordsTxt, "\n") {
		if s := strings.TrimSpace(line); s != "" && !strings.HasPrefix(s, "#") {
			m[strings.ToLower(s)] = struct{}{}
		}
	}
	return m
}()

// passwordProblem returns a validation code for a password that violates the policy,
// or "" when it is acceptable. email is used to reject passwords equal to it.
func passwordProblem(password, email string) string {
	n := utf8.RuneCountInString(password)
	switch {
	case n < minPasswordLen:
		return "too_short"
	case n > maxPasswordLen:
		return "too_long"
	}
	lower := strings.ToLower(password)
	if _, common := commonPasswords[lower]; common {
		return "too_common"
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && (lower == strings.ToLower(email) || lower == local) {
		return "matches_email"
	}
	return ""
}

// hasControlChars reports characters that must never appear in names shown to other
// people or placed in emails: control (Cc, including CR/LF) and format (Cf, including
// bidi overrides) characters.
func hasControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}
