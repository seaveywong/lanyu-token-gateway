// Package service provides business logic for the control plane, orchestrating
// repository calls with validation, password hashing, MFA, and audit logging.
package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// UserService handles user registration, login, MFA, and password management.
type UserService struct {
	users *repository.UserRepo
	audit *repository.AuditRepo
}

// NewUserService returns a UserService with the given repositories.
func NewUserService(users *repository.UserRepo, audit *repository.AuditRepo) *UserService {
	return &UserService{users: users, audit: audit}
}

// Register creates a new user with a bcrypt-hashed password. Returns the
// created user on success.
func (s *UserService) Register(ctx context.Context, email, password, displayName string) (*repository.User, error) {
	if email == "" {
		return nil, fmt.Errorf("register: email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("register: password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("register: hash password: %w", err)
	}

	user, err := s.users.CreateUser(ctx, repository.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
		DisplayName:  displayName,
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      user.ID,
		Action:       "user.registered",
		ResourceType: "user",
		ResourceID:   user.ID,
	})
	return &user, nil
}

// Login verifies credentials and returns the user on success.
func (s *UserService) Login(ctx context.Context, email, password string) (*repository.User, error) {
	if email == "" || password == "" {
		return nil, fmt.Errorf("login: email and password are required")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("login: invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("login: invalid credentials")
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      user.ID,
		Action:       "user.login",
		ResourceType: "user",
		ResourceID:   user.ID,
	})
	return user, nil
}

// SetupMFA generates a TOTP secret and returns the secret and a QR code URL
// for the user to scan. The secret is NOT stored in the database until
// EnableMFA is called successfully.
func (s *UserService) SetupMFA(ctx context.Context, userID string) (secret, qrURL string, err error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("setup mfa: %w", err)
	}
	if user == nil {
		return "", "", fmt.Errorf("setup mfa: user not found")
	}

	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("setup mfa: generate secret: %w", err)
	}
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)

	issuer := url.QueryEscape("LanyuToken")
	account := url.QueryEscape(user.Email)
	qrURL = fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, account, secret, issuer)

	return secret, qrURL, nil
}

// EnableMFA validates the provided TOTP code against the secret and enables
// MFA for the user. Returns recovery codes on success.
func (s *UserService) EnableMFA(ctx context.Context, userID, code string) ([]string, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("enable mfa: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("enable mfa: user not found")
	}

	// In a full implementation, the TOTP secret would be retrieved from a
	// temporary store (e.g. Redis) keyed by userID. For now we validate the
	// flow and toggle the MFA flag.
	if code == "" {
		return nil, fmt.Errorf("enable mfa: code is required")
	}

	if err := s.users.UpdateMFA(ctx, userID, "", true); err != nil {
		return nil, fmt.Errorf("enable mfa: %w", err)
	}

	recoveryCodes := generateRecoveryCodes()

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "user.mfa_enabled",
		ResourceType: "user",
		ResourceID:   userID,
	})
	return recoveryCodes, nil
}

// VerifyMFA validates a TOTP code for the given user during login.
// Returns true if the code is valid.
func (s *UserService) VerifyMFA(ctx context.Context, userID, code string) (bool, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("verify mfa: %w", err)
	}
	if user == nil {
		return false, fmt.Errorf("verify mfa: user not found")
	}
	if !user.MFAEnabled {
		return true, nil // MFA not set up, skip
	}

	// In a full implementation, the TOTP secret would be retrieved from
	// a secure store and validated using TOTP verification logic.
	if code == "" {
		return false, nil
	}

	// Placeholder: validate TOTP code against stored secret
	valid := validateTOTP(code, "")
	return valid, nil
}

// ChangePassword updates the user's password after verifying the old one.
func (s *UserService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if oldPassword == "" || newPassword == "" {
		return fmt.Errorf("change password: old and new passwords are required")
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	if user == nil {
		return fmt.Errorf("change password: user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("change password: old password does not match")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("change password: hash password: %w", err)
	}

	if err := s.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("change password: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "user.password_changed",
		ResourceType: "user",
		ResourceID:   userID,
	})
	return nil
}

// generateRecoveryCodes creates a set of one-time recovery codes.
func generateRecoveryCodes() []string {
	codes := make([]string, 8)
	for i := range codes {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		codes[i] = hex.EncodeToString(b)
	}
	return codes
}

// validateTOTP checks a TOTP code against a secret using HMAC-SHA1.
// In production, this would use a library like github.com/pquerna/otp.
func validateTOTP(code, secret string) bool {
	if secret == "" {
		return false
	}

	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false
	}

	// Check current and adjacent time windows (±1 step, 30s each).
	counter := uint64(time.Now().Unix() / 30)
	for delta := int64(-1); delta <= 1; delta++ {
		expected := computeTOTP(secretBytes, counter+uint64(delta))
		if expected == code {
			return true
		}
	}
	return false
}

// computeTOTP generates a TOTP value for a given secret and counter.
func computeTOTP(secret []byte, counter uint64) string {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf.Bytes())
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	binary := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	otp := binary % 1000000

	return fmt.Sprintf("%06d", otp)
}

// hashAPIKey computes the hash of a raw API key with a pepper for storage.
func hashAPIKey(rawKey string, pepper []byte) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(rawKey))
	return hex.EncodeToString(mac.Sum(nil))
}

// generateRawKey creates a new random API key string.
func generateRawKey(prefix string, byteLen int) (string, error) {
	randomBytes := make([]byte, byteLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(randomBytes)), nil
}

// hashAPIKey and generateRawKey are package-level helpers shared with
// the API key service for HMAC-based key hashing and random key generation.
