package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateRawKey returns a 32‑byte cryptographically‑random string
func GenerateRawKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashKey returns the SHA‑256 hash suitable for storage.
func HashKey(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}