package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

const secretLen = 32

// newSecret returns 32 random bytes and their base64url encoding (the value handed to
// the client in a cookie, header or email link).
func newSecret() ([]byte, string, error) {
	b := make([]byte, secretLen)
	if _, err := rand.Read(b); err != nil {
		return nil, "", fmt.Errorf("generate secret: %w", err)
	}
	return b, base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns SHA-256 of a client-held token; only this hash is stored, so a
// database leak does not reveal usable session or email tokens. Malformed input yields
// a hash that matches nothing.
func hashToken(encoded string) []byte {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != secretLen {
		raw = []byte("invalid:" + encoded)
	}
	h := sha256.Sum256(raw)
	return h[:]
}

// csrfToken is the client-visible form of a session's CSRF secret.
func csrfToken(secret []byte) string { return base64.RawURLEncoding.EncodeToString(secret) }

// validCSRF compares a submitted token to the session secret in constant time.
func validCSRF(submitted string, secret []byte) bool {
	raw, err := base64.RawURLEncoding.DecodeString(submitted)
	return err == nil && subtle.ConstantTimeCompare(raw, secret) == 1
}
