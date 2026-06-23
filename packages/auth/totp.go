package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"time"
)

const (
	// TOTPSecretLength is the recommended length for TOTP secrets (20 bytes).
	TOTPSecretLength = 20
	// TOTPDefaultDigits is the default number of digits for TOTP codes.
	TOTPDefaultDigits = 6
	// TOTPDefaultPeriod is the default time step for TOTP in seconds.
	TOTPDefaultPeriod = 30
	// RecoveryCodeCount is the number of recovery codes to generate.
	RecoveryCodeCount = 8
	// RecoveryCodeBytes is the number of random bytes per recovery code (16 hex chars).
	RecoveryCodeBytes = 8
)

// GenerateTOTPSecret creates a new TOTP secret and returns the base32-encoded
// secret along with an otpauth:// URL suitable for QR code generation.
func GenerateTOTPSecret(issuer, accountName string) (secret string, qrCodeURL string, err error) {
	rawSecret := make([]byte, TOTPSecretLength)
	if _, err := rand.Read(rawSecret); err != nil {
		return "", "", fmt.Errorf("generate totp secret: %w", err)
	}

	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rawSecret)

	// Build the otpauth URL: otpauth://totp/<issuer>:<account>?secret=<secret>&issuer=<issuer>
	label := url.QueryEscape(issuer + ":" + accountName)
	qrCodeURL = fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		label,
		url.QueryEscape(secret),
		url.QueryEscape(issuer),
		TOTPDefaultDigits,
		TOTPDefaultPeriod,
	)

	return secret, qrCodeURL, nil
}

// ValidateTOTP validates a TOTP code against the given secret.
// It checks the code against the current time step and ±1 adjacent steps.
func ValidateTOTP(secret, code string) (bool, error) {
	if secret == "" || code == "" {
		return false, nil
	}

	rawSecret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false, fmt.Errorf("decode totp secret: %w", err)
	}

	now := time.Now().Unix()
	counter := uint64(now / TOTPDefaultPeriod)

	// Check current and adjacent time windows (±1 step)
	for offset := int64(-1); offset <= 1; offset++ {
		expectedCode := computeTOTP(rawSecret, counter+uint64(offset), TOTPDefaultDigits)
		if hmac.Equal([]byte(code), []byte(expectedCode)) {
			return true, nil
		}
	}

	return false, nil
}

// computeTOTP computes a TOTP code for the given secret, counter, and digit count.
// This implements RFC 6238 (TOTP) which is built on RFC 4226 (HOTP).
func computeTOTP(secret []byte, counter uint64, digits int) string {
	h := hmac.New(sha1.New, secret)
	binary.Write(h, binary.BigEndian, counter)
	hash := h.Sum(nil)

	// Dynamic truncation per RFC 4226 section 5.4
	offset := hash[len(hash)-1] & 0x0f
	binary := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	modulus := uint32(math.Pow10(digits))
	otp := binary % modulus

	// Zero-pad to the requested number of digits
	return fmt.Sprintf("%0*d", digits, otp)
}

// GenerateRecoveryCodes creates a set of recovery codes for account recovery
// when TOTP is unavailable. Each code is 16 hex characters (8 bytes of entropy).
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		b := make([]byte, RecoveryCodeBytes)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate recovery code %d: %w", i, err)
		}
		codes[i] = fmt.Sprintf("%x", b)
	}
	return codes, nil
}
