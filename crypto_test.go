package main

import (
	"os"
	"strings"
	"testing"
)

const testKey32 = "0123456789abcdef0123456789abcdef"

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	plaintext := "Hello, Lavender!"
	encrypted, err := encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if string(encrypted) == plaintext {
		t.Error("encrypted text should differ from plaintext")
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("decrypted %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	encrypted, err := encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty string failed: %v", err)
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptDecrypt_Unicode(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	plaintext := "Привет мир! 🌍 日本語"
	encrypted, err := encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt unicode failed: %v", err)
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt unicode failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("unicode round-trip failed: got %q", decrypted)
	}
}

func TestEncrypt_WrongKeyLength(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", "short")

	_, err := encrypt("test")
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestDecrypt_WrongKeyLength(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", "short")

	_, err := decrypt([]byte("some-ciphertext"))
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestDecrypt_TooShortCiphertext(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	_, err := decrypt([]byte("short"))
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	encrypted, err := encrypt("secret message")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("encrypted should not be empty")
	}
	encrypted[len(encrypted)-1] ^= 0xFF

	_, err = decrypt(encrypted)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestDecrypt_ServiceMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"SERVICE_VOICE_MSG", "Voice message"},
		{"SERVICE_MEDIA_MSG", "Image"},
		{"FIXED_BY_MAINTENANCE", "Message"},
		{"CORRUPTED_FIX", "Message"},
		{"EMPTY_FIX", "Message"},
	}
	for _, tt := range tests {
		got, err := decrypt([]byte(tt.input))
		if err != nil {
			t.Errorf("decrypt(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("decrypt(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEncrypt_DifferentCiphertextEachTime(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	enc1, _ := encrypt("same message")
	enc2, _ := encrypt("same message")
	if string(enc1) == string(enc2) {
		t.Error("two encryptions of same plaintext should produce different ciphertexts (random nonce)")
	}
}

// ======= HashPassword / CheckPassword =======

func TestHashPassword_CheckPassword_Success(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "mypassword" {
		t.Error("hash should not equal plaintext")
	}

	if !CheckPassword("mypassword", hash) {
		t.Error("CheckPassword should return true for correct password")
	}
}

func TestHashPassword_CheckPassword_WrongPassword(t *testing.T) {
	t.Parallel()
	hash, _ := HashPassword("correct-password")
	if CheckPassword("wrong-password", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestHashPassword_CheckPassword_EmptyPassword(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword empty failed: %v", err)
	}
	if !CheckPassword("", hash) {
		t.Error("CheckPassword should return true for empty password matching empty hash")
	}
}

func TestHashPassword_DifferentHashesSamePassword(t *testing.T) {
	t.Parallel()
	hash1, _ := HashPassword("test")
	hash2, _ := HashPassword("test")
	if hash1 == hash2 {
		t.Error("bcrypt should produce different hashes for same password (random salt)")
	}
}

func TestHashPassword_HashFormat(t *testing.T) {
	t.Parallel()
	hash, _ := HashPassword("test")
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("bcrypt hash should start with $2a$ or $2b$, got prefix: %s", hash[:4])
	}
}

// ======= GenerateResetToken =======

func TestGenerateResetToken_Length(t *testing.T) {
	t.Parallel()
	token, err := GenerateResetToken()
	if err != nil {
		t.Fatalf("GenerateResetToken failed: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(token))
	}
}

func TestGenerateResetToken_HexFormat(t *testing.T) {
	t.Parallel()
	token, _ := GenerateResetToken()
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("unexpected char in hex token: %c", c)
			break
		}
	}
}

func TestGenerateResetToken_Unique(t *testing.T) {
	t.Parallel()
	t1, _ := GenerateResetToken()
	t2, _ := GenerateResetToken()
	if t1 == t2 {
		t.Error("two tokens should be unique")
	}
}

// ======= getSecretKey =======

func TestGetSecretKey_Valid(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", testKey32)

	key, err := getSecretKey()
	if err != nil {
		t.Fatalf("getSecretKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(key))
	}
}

func TestGetSecretKey_TooShort(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", "short")

	_, err := getSecretKey()
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestGetSecretKey_Empty(t *testing.T) {
	os.Unsetenv("CHAT_SECRET_KEY")

	_, err := getSecretKey()
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestGetSecretKey_TooLong(t *testing.T) {
	t.Setenv("CHAT_SECRET_KEY", "0123456789abcdef0123456789abcdefextra")

	_, err := getSecretKey()
	if err == nil {
		t.Error("expected error for key > 32 bytes")
	}
}
