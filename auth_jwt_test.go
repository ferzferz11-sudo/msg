package main

import (
	"os"
	"testing"
	"time"
)

func TestGenerateAndValidateTokenPair(t *testing.T) {
	// Save and restore env
	origSecret := os.Getenv("JWT_SECRET")
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Setenv("JWT_SECRET", origSecret)

	userID := "test-user-123"
	username := "testuser"
	deviceID := "test-device-456"

	// Generate token pair
	accessToken, refreshToken, accessExp, refreshExp, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if accessToken == "" {
		t.Fatal("Access token is empty")
	}
	if refreshToken == "" {
		t.Fatal("Refresh token is empty")
	}
	if accessExp.Before(time.Now()) {
		t.Fatal("Access token already expired")
	}
	if refreshExp.Before(time.Now()) {
		t.Fatal("Refresh token already expired")
	}

	// Refresh should be longer-lived than access
	if refreshExp.Sub(accessExp) < AccessTokenTTL {
		t.Fatal("Refresh token should live longer than access token")
	}

	// Validate access token
	claims, err := ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken (access) failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID mismatch: got %s, want %s", claims.UserID, userID)
	}
	if claims.Username != username {
		t.Errorf("Username mismatch: got %s, want %s", claims.Username, username)
	}
	if claims.DeviceID != deviceID {
		t.Errorf("DeviceID mismatch: got %s, want %s", claims.DeviceID, deviceID)
	}
	if claims.Type != "access" {
		t.Errorf("Type mismatch: got %s, want access", claims.Type)
	}

	// Validate refresh token
	refreshClaims, err := ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateToken (refresh) failed: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Errorf("Type mismatch: got %s, want refresh", refreshClaims.Type)
	}
	if refreshClaims.UserID != userID {
		t.Errorf("Refresh UserID mismatch: got %s, want %s", refreshClaims.UserID, userID)
	}

	// Extract JTI
	jti, err := ExtractJTI(refreshToken)
	if err != nil {
		t.Fatalf("ExtractJTI failed: %v", err)
	}
	if jti == "" {
		t.Fatal("JTI is empty")
	}
	if jti != refreshClaims.ID {
		t.Errorf("JTI mismatch: ExtractJTI=%s, claims.ID=%s", jti, refreshClaims.ID)
	}

	// Tampered token should fail
	_, err = ValidateToken(accessToken + "tampered")
	if err == nil {
		t.Fatal("Tampered token should fail validation")
	}

	// Wrong secret should fail
	os.Setenv("JWT_SECRET", "different-secret-that-is-32-bytes-long")
	_, err = ValidateToken(accessToken)
	if err == nil {
		t.Fatal("Token with wrong secret should fail validation")
	}
}

func TestValidateTokenExpired(t *testing.T) {
	origSecret := os.Getenv("JWT_SECRET")
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Setenv("JWT_SECRET", origSecret)

	// Generate token
	accessToken, _, _, _, err := GenerateTokenPair("user1", "testuser", "dev1")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	// Token should be valid now
	_, err = ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("Fresh token should be valid: %v", err)
	}
}
