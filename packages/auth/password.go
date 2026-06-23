package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// DefaultArgon2Params holds the default parameters for Argon2id password hashing.
var DefaultArgon2Params = Argon2Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// Argon2Params configures the Argon2id key derivation function.
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// HashPassword hashes a password using Argon2id with the default parameters.
// The returned string uses the format:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}

	salt := make([]byte, DefaultArgon2Params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		DefaultArgon2Params.Iterations,
		DefaultArgon2Params.Memory,
		DefaultArgon2Params.Parallelism,
		DefaultArgon2Params.KeyLength,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		DefaultArgon2Params.Memory,
		DefaultArgon2Params.Iterations,
		DefaultArgon2Params.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// VerifyPassword compares a plaintext password against an encoded Argon2id hash.
// It returns true if the password matches, false otherwise.
func VerifyPassword(password, encodedHash string) (bool, error) {
	if password == "" || encodedHash == "" {
		return false, nil
	}

	params, salt, hash, err := decodeArgon2Hash(encodedHash)
	if err != nil {
		return false, err
	}

	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	return subtle.ConstantTimeCompare(hash, computedHash) == 1, nil
}

// decodeArgon2Hash parses an encoded Argon2id hash string and returns the
// parameters, salt, and hash bytes.
func decodeArgon2Hash(encoded string) (Argon2Params, []byte, []byte, error) {
	// Expected format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return Argon2Params{}, nil, nil, fmt.Errorf("invalid argon2 hash format: expected 6 parts, got %d", len(parts))
	}

	if parts[1] != "argon2id" {
		return Argon2Params{}, nil, nil, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("parse version: %w", err)
	}
	if version != 19 {
		return Argon2Params{}, nil, nil, fmt.Errorf("unsupported argon2 version: %d", version)
	}

	var params Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}

	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(hash))

	return params, salt, hash, nil
}
