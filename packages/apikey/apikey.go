package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// KeyPrefixLive is the prefix for live (production) API keys.
	KeyPrefixLive = "ly_live_"
	// KeyPrefixTest is the prefix for test API keys.
	KeyPrefixTest = "ly_test_"
	// KeyRandomBytes is the number of CSPRNG bytes used for the random key part.
	KeyRandomBytes = 32
)

// GeneratedKey holds the components of a newly generated API key.
type GeneratedKey struct {
	// RawKey is the full API key string, e.g. "ly_live_XyZabc...".
	RawKey string
	// KeyPrefix is the prefix of the key, e.g. "ly_live_".
	KeyPrefix string
	// KeyHash is the HMAC-SHA-256 hash of the raw key, hex-encoded.
	KeyHash string
	// KeyFingerprint is a human-identifiable fingerprint: first 8 + last 4 chars.
	KeyFingerprint string
}

// Generate creates a new API key for the given environment ("live" or "test").
// Returns a GeneratedKey containing the raw key, prefix, hash, and fingerprint.
func Generate(env string) (*GeneratedKey, error) {
	var prefix string
	switch env {
	case "live":
		prefix = KeyPrefixLive
	case "test":
		prefix = KeyPrefixTest
	default:
		return nil, fmt.Errorf("invalid env %q: must be \"live\" or \"test\"", env)
	}

	randomBytes := make([]byte, KeyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}

	// Base64url encode without padding
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)
	rawKey := prefix + encoded

	fingerprint := makeFingerprint(rawKey)

	return &GeneratedKey{
		RawKey:         rawKey,
		KeyPrefix:      prefix,
		KeyHash:        "", // Caller must hash with HashKey
		KeyFingerprint: fingerprint,
	}, nil
}

// HashKey computes the HMAC-SHA-256 hash of a raw API key using the given pepper.
// Returns the hex-encoded hash and a human-readable fingerprint.
func HashKey(pepper []byte, rawKey string) (hash string, fingerprint string, err error) {
	if len(pepper) == 0 {
		return "", "", fmt.Errorf("pepper must not be empty")
	}
	if rawKey == "" {
		return "", "", fmt.Errorf("rawKey must not be empty")
	}

	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(rawKey))
	hash = hex.EncodeToString(mac.Sum(nil))
	fingerprint = makeFingerprint(rawKey)

	return hash, fingerprint, nil
}

// ValidateKey verifies a raw API key against a stored HMAC hash using constant-time comparison.
func ValidateKey(pepper []byte, rawKey, storedHash string) (bool, error) {
	if len(pepper) == 0 {
		return false, fmt.Errorf("pepper must not be empty")
	}
	if rawKey == "" || storedHash == "" {
		return false, nil
	}

	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(rawKey))
	computedHash := hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedHash)) == 1, nil
}

// ExtractPrefix extracts the key prefix ("ly_live_" or "ly_test_") from a raw API key.
// Returns an empty string if no known prefix is found.
func ExtractPrefix(rawKey string) string {
	if strings.HasPrefix(rawKey, KeyPrefixLive) {
		return KeyPrefixLive
	}
	if strings.HasPrefix(rawKey, KeyPrefixTest) {
		return KeyPrefixTest
	}
	return ""
}

// MaskKey returns a masked version of the API key for display purposes.
// Shows the first 12 characters, then "...", then the last 4 characters.
func MaskKey(rawKey string) string {
	if len(rawKey) <= 16 {
		return rawKey
	}
	return rawKey[:12] + "..." + rawKey[len(rawKey)-4:]
}

// makeFingerprint creates a short fingerprint from the raw key:
// first 8 characters + last 4 characters.
func makeFingerprint(rawKey string) string {
	if len(rawKey) <= 12 {
		return rawKey
	}
	return rawKey[:8] + rawKey[len(rawKey)-4:]
}
