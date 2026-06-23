package auth

import (
	"testing"
)

func TestHashPassword_Success(t *testing.T) {
	hash, err := HashPassword("my-secret-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}
}

func TestHashPassword_Empty(t *testing.T) {
	_, err := HashPassword("")
	if err == nil {
		t.Fatal("HashPassword() with empty password should return error")
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword() should return true for correct password")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword() should return false for wrong password")
	}
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	hash, err := HashPassword("some-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := VerifyPassword("", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword() should return false for empty password")
	}
}

func TestVerifyPassword_EmptyHash(t *testing.T) {
	ok, err := VerifyPassword("password", "")
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword() should return false for empty hash")
	}
}

func TestVerifyPassword_EachHashUnique(t *testing.T) {
	password := "same-password"
	hashes := make(map[string]bool)

	for i := 0; i < 10; i++ {
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}
		if hashes[hash] {
			t.Fatal("HashPassword() produced duplicate hash")
		}
		hashes[hash] = true

		ok, err := VerifyPassword(password, hash)
		if err != nil {
			t.Fatalf("VerifyPassword() error = %v", err)
		}
		if !ok {
			t.Fatal("VerifyPassword() should return true")
		}
	}
}

func TestDefaultArgon2Params(t *testing.T) {
	if DefaultArgon2Params.Memory != 64*1024 {
		t.Errorf("expected Memory=65536, got %d", DefaultArgon2Params.Memory)
	}
	if DefaultArgon2Params.Iterations != 3 {
		t.Errorf("expected Iterations=3, got %d", DefaultArgon2Params.Iterations)
	}
	if DefaultArgon2Params.Parallelism != 4 {
		t.Errorf("expected Parallelism=4, got %d", DefaultArgon2Params.Parallelism)
	}
	if DefaultArgon2Params.SaltLength != 16 {
		t.Errorf("expected SaltLength=16, got %d", DefaultArgon2Params.SaltLength)
	}
	if DefaultArgon2Params.KeyLength != 32 {
		t.Errorf("expected KeyLength=32, got %d", DefaultArgon2Params.KeyLength)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword("benchmark-password")
	}
}
