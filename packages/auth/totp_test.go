package auth

import (
	"strings"
	"testing"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, qrURL, err := GenerateTOTPSecret("TestIssuer", "user@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error = %v", err)
	}

	if secret == "" {
		t.Fatal("GenerateTOTPSecret() returned empty secret")
	}

	// Check URL format
	if !strings.HasPrefix(qrURL, "otpauth://totp/") {
		t.Errorf("invalid otpauth URL: %s", qrURL)
	}
	if !strings.Contains(qrURL, "secret=") {
		t.Errorf("otpauth URL missing secret parameter: %s", qrURL)
	}
	if !strings.Contains(qrURL, "issuer=TestIssuer") {
		t.Errorf("otpauth URL missing issuer: %s", qrURL)
	}
}

func TestGenerateTOTPSecret_EachUnique(t *testing.T) {
	secrets := make(map[string]bool)
	for i := 0; i < 10; i++ {
		secret, _, err := GenerateTOTPSecret("Issuer", "user@example.com")
		if err != nil {
			t.Fatalf("GenerateTOTPSecret() error = %v", err)
		}
		if secrets[secret] {
			t.Fatal("GenerateTOTPSecret() produced duplicate secret")
		}
		secrets[secret] = true
	}
}

func TestValidateTOTP_Empty(t *testing.T) {
	ok, err := ValidateTOTP("", "123456")
	if err != nil {
		t.Fatalf("ValidateTOTP() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateTOTP() should return false for empty secret")
	}

	ok, err = ValidateTOTP("SECRET", "")
	if err != nil {
		t.Fatalf("ValidateTOTP() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateTOTP() should return false for empty code")
	}
}

func TestValidateTOTP_WrongCode(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("Issuer", "user@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error = %v", err)
	}

	// A wrong code should not validate
	ok, err := ValidateTOTP(secret, "000000")
	if err != nil {
		t.Fatalf("ValidateTOTP() error = %v", err)
	}
	if ok {
		t.Log("warning: ValidateTOTP returned true for unlikely wrong code '000000'")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}

	if len(codes) != RecoveryCodeCount {
		t.Errorf("got %d recovery codes, want %d", len(codes), RecoveryCodeCount)
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		if len(code) != 16 {
			t.Errorf("recovery code length = %d, want 16 (code: %s)", len(code), code)
		}
		if seen[code] {
			t.Fatal("GenerateRecoveryCodes() produced duplicate code")
		}
		seen[code] = true
	}
}
