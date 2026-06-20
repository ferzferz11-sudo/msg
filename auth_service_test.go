package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// mockAuthDB implements authDB for testing
type mockAuthDB struct {
	users  map[string]*mockUser
	emails map[string]string
}

type mockUser struct {
	username  string
	hash      string
	email     string
	id        string
	createdAt time.Time
}

func newMockAuthDB() *mockAuthDB {
	return &mockAuthDB{
		users:  make(map[string]*mockUser),
		emails: make(map[string]string),
	}
}

func (m *mockAuthDB) UserExists(user string) (bool, error) {
	_, ok := m.users[user]
	return ok, nil
}

func (m *mockAuthDB) EmailExists(email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	_, ok := m.emails[email]
	return ok, nil
}

func (m *mockAuthDB) GetUserPasswordHash(user string) (string, error) {
	u, ok := m.users[user]
	if !ok {
		return "", fmt.Errorf("user not found")
	}
	return u.hash, nil
}

func (m *mockAuthDB) SaveUserWithEmail(user, hash, email string) error {
	m.users[user] = &mockUser{
		username:  user,
		hash:      hash,
		email:     email,
		id:        "test-" + user,
		createdAt: time.Now(),
	}
	if email != "" {
		m.emails[email] = user
	}
	return nil
}

func (m *mockAuthDB) GetUserIdByUsername(user string) (string, error) {
	u, ok := m.users[user]
	if !ok {
		return "", fmt.Errorf("user not found")
	}
	return u.id, nil
}

func (m *mockAuthDB) GetUserAvatar(user string) (string, error) {
	return "", nil
}

func (m *mockAuthDB) UpdateLastSeen(user string) error {
	return nil
}

func (m *mockAuthDB) queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error) {
	u, ok := m.users[username]
	if !ok {
		err = fmt.Errorf("user not found")
		return
	}
	return u.email, "", "", u.createdAt, time.Now(), nil
}

func (m *mockAuthDB) addUser(username, hash, email string) {
	m.users[username] = &mockUser{
		username:  username,
		hash:      hash,
		email:     email,
		id:        "test-" + username,
		createdAt: time.Now(),
	}
	if email != "" {
		m.emails[email] = username
	}
}

func (m *mockAuthDB) UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, ipAddress, userAgent string) (*UserDevice, error) {
	return &UserDevice{ID: "dev-" + deviceID, UserID: userID, DeviceID: deviceID, DeviceName: deviceName}, nil
}
func (m *mockAuthDB) UpdateDeviceRefreshToken(userID, deviceID, jti string, expiresAt time.Time) error {
	return nil
}
func (m *mockAuthDB) GetDevices(userID string) ([]UserDevice, error) {
	return nil, nil
}
func (m *mockAuthDB) RevokeDevice(userID, deviceID string) error {
	return nil
}
func (m *mockAuthDB) RevokeAllDevices(userID string) error {
	return nil
}
func (m *mockAuthDB) IsDeviceActive(userID, deviceID string) (bool, error) {
	return true, nil
}
func (m *mockAuthDB) ValidateRefreshToken(userID, deviceID, jti string) (bool, error) {
	return true, nil
}
func (m *mockAuthDB) UpdateDeviceLastSeen(userID, deviceID string) error {
	return nil
}
func (m *mockAuthDB) LogAuthEvent(userID, deviceID, action, ipAddress, clientVersion string, success bool, errorMessage string) {
}

// ===== V2 Auth Tests =====

func TestSignInV2_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("testuser", hash, "test@example.com")

	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignInV2(context.Background(), &gen.SignInRequestV2{
		Username: "testuser",
		Password: "pass123",
		Device: &gen.DeviceInfo{
			DeviceId:   "device-123",
			DeviceName: "Test Device",
		},
	})

	if err != nil {
		t.Fatalf("SignInV2 returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if resp.User == nil {
		t.Fatal("expected non-nil user")
	}
	if resp.User.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", resp.User.Username)
	}
}

func TestSignInV2_WrongPassword(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("testuser", hash, "")

	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignInV2(context.Background(), &gen.SignInRequestV2{
		Username: "testuser",
		Password: "wrongpass",
		Device: &gen.DeviceInfo{
			DeviceId:   "device-123",
			DeviceName: "Test Device",
		},
	})

	if err != nil {
		t.Fatalf("SignInV2 returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for wrong password")
	}
}

func TestSignInV2_EmptyCredentials(t *testing.T) {
	db := newMockAuthDB()
	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignInV2(context.Background(), &gen.SignInRequestV2{
		Username: "",
		Password: "",
	})

	if err != nil {
		t.Fatalf("SignInV2 returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for empty credentials")
	}
}

func TestSignInV2_UserNotFound(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	db := newMockAuthDB()
	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignInV2(context.Background(), &gen.SignInRequestV2{
		Username: "nonexistent",
		Password: "pass123",
		Device: &gen.DeviceInfo{
			DeviceId: "device-123",
		},
	})

	if err != nil {
		t.Fatalf("SignInV2 returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for nonexistent user")
	}
}

func TestSignUpV2_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	db := newMockAuthDB()
	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignUpV2(context.Background(), &gen.SignUpRequestV2{
		Username: "newuser",
		Password: "pass123",
		Device: &gen.DeviceInfo{
			DeviceId:   "device-456",
			DeviceName: "New Device",
		},
	})

	if err != nil {
		t.Fatalf("SignUpV2 returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
}

func TestSignUpV2_DuplicateUsername(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("existing", hash, "")

	srv := newAuthServerV2(nil)
	srv.db = db

	resp, err := srv.SignUpV2(context.Background(), &gen.SignUpRequestV2{
		Username: "existing",
		Password: "newpass",
		Device: &gen.DeviceInfo{
			DeviceId: "device-789",
		},
	})

	if err != nil {
		t.Fatalf("SignUpV2 returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for duplicate username")
	}
}

func TestTokenPair_Validation(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
	defer os.Unsetenv("JWT_SECRET")

	userID := "test-user-uuid-123"
	username := "testuser"
	deviceID := "device-abc"

	accessToken, refreshToken, _, _, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	accessClaims, err := ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken (access) failed: %v", err)
	}
	if accessClaims.UserID != userID {
		t.Errorf("Access token UserID mismatch: got %s, want %s", accessClaims.UserID, userID)
	}
	if accessClaims.Username != username {
		t.Errorf("Access token Username mismatch: got %s, want %s", accessClaims.Username, username)
	}
	if accessClaims.DeviceID != deviceID {
		t.Errorf("Access token DeviceID mismatch: got %s, want %s", accessClaims.DeviceID, deviceID)
	}
	if accessClaims.Type != "access" {
		t.Errorf("Access token Type mismatch: got %s, want access", accessClaims.Type)
	}

	refreshClaims, err := ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateToken (refresh) failed: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Errorf("Refresh token Type mismatch: got %s, want refresh", refreshClaims.Type)
	}
	if refreshClaims.UserID != userID {
		t.Errorf("Refresh token UserID mismatch: got %s, want %s", refreshClaims.UserID, userID)
	}

	_, err = ValidateToken(accessToken + "tampered")
	if err == nil {
		t.Fatal("Tampered token should fail validation")
	}

	os.Setenv("JWT_SECRET", "different-secret-that-is-32-bytes-long")
	_, err = ValidateToken(accessToken)
	if err == nil {
		t.Fatal("Token with wrong secret should fail validation")
	}
}
