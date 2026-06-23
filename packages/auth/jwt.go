package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims extends the standard JWT registered claims with application-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	UserID string   `json:"user_id"`
	OrgID  string   `json:"org_id,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

// GenerateAccessToken creates a signed JWT access token with user identity and roles.
// It includes exp, iat, and jti claims and is signed with HS256.
func GenerateAccessToken(userID, orgID string, roles []string, secret []byte, expiry time.Duration) (string, error) {
	now := time.Now()
	jti := uuid.NewString()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
		UserID: userID,
		OrgID:  orgID,
		Roles:  roles,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// GenerateRefreshToken creates a signed JWT refresh token without roles.
// It includes exp, iat, and jti claims and is signed with HS256.
func GenerateRefreshToken(userID string, secret []byte, expiry time.Duration) (string, error) {
	now := time.Now()
	jti := uuid.NewString()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateToken parses and validates a JWT token string.
// It returns the parsed Claims if the token is valid.
func ValidateToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// GenerateTokenPair creates both an access token and a refresh token in one call.
func GenerateTokenPair(userID, orgID string, roles []string, secret []byte, accessExpiry, refreshExpiry time.Duration) (accessToken, refreshToken string, err error) {
	accessToken, err = GenerateAccessToken(userID, orgID, roles, secret, accessExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err = GenerateRefreshToken(userID, secret, refreshExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}
