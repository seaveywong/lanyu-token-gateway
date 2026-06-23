package auth

import (
	"testing"
	"time"
)

var testSecret = []byte("test-secret-key-for-jwt-signing")

func TestGenerateAccessToken_Validate(t *testing.T) {
	userID := "user-123"
	orgID := "org-456"
	roles := []string{"admin", "editor"}

	token, err := GenerateAccessToken(userID, orgID, roles, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateAccessToken() returned empty token")
	}

	claims, err := ValidateToken(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %q, want %q", claims.UserID, userID)
	}
	if claims.OrgID != orgID {
		t.Errorf("OrgID = %q, want %q", claims.OrgID, orgID)
	}
	if len(claims.Roles) != len(roles) {
		t.Errorf("got %d roles, want %d", len(claims.Roles), len(roles))
	}
	if claims.ID == "" {
		t.Error("jti should not be empty")
	}
}

func TestExpiredToken(t *testing.T) {
	userID := "user-123"

	// Generate a token that expires immediately (negative duration)
	token, err := GenerateAccessToken(userID, "", nil, testSecret, -1*time.Second)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	_, err = ValidateToken(token, testSecret)
	if err == nil {
		t.Fatal("ValidateToken() should return error for expired token")
	}
}

func TestWrongSecret(t *testing.T) {
	userID := "user-123"

	token, err := GenerateAccessToken(userID, "", nil, testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	wrongSecret := []byte("wrong-secret-key")
	_, err = ValidateToken(token, wrongSecret)
	if err == nil {
		t.Fatal("ValidateToken() should return error with wrong secret")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	userID := "user-123"

	token, err := GenerateRefreshToken(userID, testSecret, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateRefreshToken() returned empty token")
	}

	claims, err := ValidateToken(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %q, want %q", claims.UserID, userID)
	}
	if claims.Roles != nil {
		t.Errorf("Refresh token should not contain roles, got %v", claims.Roles)
	}
}

func TestGenerateTokenPair(t *testing.T) {
	userID := "user-123"
	orgID := "org-456"
	roles := []string{"viewer"}

	access, refresh, err := GenerateTokenPair(userID, orgID, roles, testSecret, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("GenerateTokenPair() returned empty token(s)")
	}
	if access == refresh {
		t.Fatal("access and refresh tokens should be different")
	}

	// Validate both tokens
	aClaims, err := ValidateToken(access, testSecret)
	if err != nil {
		t.Fatalf("ValidateToken(access) error = %v", err)
	}
	if aClaims.UserID != userID {
		t.Errorf("access UserID = %q, want %q", aClaims.UserID, userID)
	}

	rClaims, err := ValidateToken(refresh, testSecret)
	if err != nil {
		t.Fatalf("ValidateToken(refresh) error = %v", err)
	}
	if rClaims.UserID != userID {
		t.Errorf("refresh UserID = %q, want %q", rClaims.UserID, userID)
	}
}
