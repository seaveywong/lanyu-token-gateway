package apikey

import (
	"strings"
	"testing"
)

var testPepper = []byte("test-pepper-for-hmac")

func TestGenerate_LiveKey(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate(live) error = %v", err)
	}

	if key.KeyPrefix != KeyPrefixLive {
		t.Errorf("KeyPrefix = %q, want %q", key.KeyPrefix, KeyPrefixLive)
	}
	if !strings.HasPrefix(key.RawKey, KeyPrefixLive) {
		t.Errorf("RawKey should start with %q: got %q", KeyPrefixLive, key.RawKey)
	}
	if key.RawKey == KeyPrefixLive {
		t.Fatal("RawKey should have random content after prefix")
	}
	if key.KeyFingerprint == "" {
		t.Fatal("KeyFingerprint should not be empty")
	}
}

func TestGenerate_TestKey(t *testing.T) {
	key, err := Generate("test")
	if err != nil {
		t.Fatalf("Generate(test) error = %v", err)
	}

	if key.KeyPrefix != KeyPrefixTest {
		t.Errorf("KeyPrefix = %q, want %q", key.KeyPrefix, KeyPrefixTest)
	}
	if !strings.HasPrefix(key.RawKey, KeyPrefixTest) {
		t.Errorf("RawKey should start with %q: got %q", KeyPrefixTest, key.RawKey)
	}
}

func TestGenerate_InvalidEnv(t *testing.T) {
	_, err := Generate("staging")
	if err == nil {
		t.Fatal("Generate(staging) should return error")
	}
	_, err = Generate("")
	if err == nil {
		t.Fatal("Generate(\"\") should return error")
	}
}

func TestGenerate_EachUnique(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key, err := Generate("live")
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if keys[key.RawKey] {
			t.Fatal("Generate() produced duplicate key")
		}
		keys[key.RawKey] = true
	}
}

func TestHashKey_RoundTrip(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	hash, fp, err := HashKey(testPepper, key.RawKey)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashKey() returned empty hash")
	}
	if fp == "" {
		t.Fatal("HashKey() returned empty fingerprint")
	}

	ok, err := ValidateKey(testPepper, key.RawKey, hash)
	if err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateKey() should return true for correct key")
	}
}

func TestValidateKey_WrongKey(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	hash, _, err := HashKey(testPepper, key.RawKey)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}

	ok, err := ValidateKey(testPepper, "ly_live_wrong-key-value", hash)
	if err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateKey() should return false for wrong key")
	}
}

func TestValidateKey_WrongPepper(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	hash, _, err := HashKey(testPepper, key.RawKey)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}

	wrongPepper := []byte("wrong-pepper")
	ok, err := ValidateKey(wrongPepper, key.RawKey, hash)
	if err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateKey() should return false with wrong pepper")
	}
}

func TestValidateKey_EmptyInputs(t *testing.T) {
	ok, err := ValidateKey(testPepper, "", "some-hash")
	if err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateKey() should return false for empty rawKey")
	}

	ok, err = ValidateKey(testPepper, "some-key", "")
	if err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateKey() should return false for empty storedHash")
	}
}

func TestHashKey_EmptyArgs(t *testing.T) {
	_, _, err := HashKey([]byte{}, "some-key")
	if err == nil {
		t.Fatal("HashKey() should return error for empty pepper")
	}

	_, _, err = HashKey(testPepper, "")
	if err == nil {
		t.Fatal("HashKey() should return error for empty rawKey")
	}
}

func TestMaskKey(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	masked := MaskKey(key.RawKey)
	if !strings.Contains(masked, "...") {
		t.Errorf("MaskKey() = %q, should contain \"...\"", masked)
	}
	if len(masked) != 12+3+4 {
		t.Errorf("MaskKey() length = %d, want %d", len(masked), 12+3+4)
	}
}

func TestMaskKey_Short(t *testing.T) {
	short := "abc"
	if MaskKey(short) != "abc" {
		t.Errorf("MaskKey(short) should return unchanged: got %q", MaskKey(short))
	}
}

func TestExtractPrefix_Live(t *testing.T) {
	key, err := Generate("live")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	prefix := ExtractPrefix(key.RawKey)
	if prefix != KeyPrefixLive {
		t.Errorf("ExtractPrefix() = %q, want %q", prefix, KeyPrefixLive)
	}
}

func TestExtractPrefix_Test(t *testing.T) {
	key, err := Generate("test")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	prefix := ExtractPrefix(key.RawKey)
	if prefix != KeyPrefixTest {
		t.Errorf("ExtractPrefix() = %q, want %q", prefix, KeyPrefixTest)
	}
}

func TestExtractPrefix_Unknown(t *testing.T) {
	if prefix := ExtractPrefix("bad_xxx_key"); prefix != "" {
		t.Errorf("ExtractPrefix() = %q, want empty string", prefix)
	}
}

func TestHashKey_Deterministic(t *testing.T) {
	rawKey := "ly_live_fixed-test-key"
	hash1, _, err := HashKey(testPepper, rawKey)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}
	hash2, _, err := HashKey(testPepper, rawKey)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}
	if hash1 != hash2 {
		t.Fatal("HashKey() should be deterministic for same inputs")
	}
}

func TestGenerate_RandomHashUniqueness(t *testing.T) {
	hashes := make(map[string]bool)
	n := 1000
	for i := 0; i < n; i++ {
		key, err := Generate("live")
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		hash, _, err := HashKey(testPepper, key.RawKey)
		if err != nil {
			t.Fatalf("HashKey() error = %v", err)
		}
		if hashes[hash] {
			t.Fatal("HashKey() produced duplicate hash across different keys")
		}
		hashes[hash] = true
	}
	t.Logf("Generated %d unique hashes successfully", n)
}
