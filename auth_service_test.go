package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"testing"
	"time"
)

// mockAuthDB implements authDB for testing
type mockAuthDB struct {
	users       map[string]*mockUser // username -> user
	emails      map[string]string    // email -> username
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

// V2 mock methods
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

// ===== SignIn tests =====

func TestSignIn_Success(t *testing.T) {
	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("testuser", hash, "test@example.com")

	srv := &authServer{db: db}
	resp, err := srv.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "pass123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User == nil {
		t.Fatal("expected non-nil user")
	}
	if resp.User.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", resp.User.Username)
	}
}

func TestSignIn_WrongPassword(t *testing.T) {
	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("testuser", hash, "")

	srv := &authServer{db: db}
	resp, err := srv.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "wrongpass",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for wrong password")
	}
	if resp.Message == "" {
		t.Error("expected error message")
	}
}

func TestSignIn_UserNotFound(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignIn(context.Background(), &gen.SignInRequest{
		Username: "nonexistent",
		Password: "pass123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for nonexistent user")
	}
}

func TestSignIn_EmptyUsername(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignIn(context.Background(), &gen.SignInRequest{
		Username: "",
		Password: "pass123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for empty username")
	}
}

func TestSignIn_EmptyPassword(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for empty password")
	}
}

// ===== SignUp tests =====

func TestSignUp_Success(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "pass123",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestSignUp_DuplicateUsername(t *testing.T) {
	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("existing", hash, "")

	srv := &authServer{db: db}
	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "existing",
		Password: "newpass",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for duplicate username")
	}
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("user1", hash, "same@example.com")

	srv := &authServer{db: db}
	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "user2",
		Password: "pass456",
		Email:    "same@example.com",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for duplicate email")
	}
}

func TestSignUp_EmptyUsername(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "",
		Password: "pass123",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for empty username")
	}
}

func TestSignUp_EmptyPassword(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for empty password")
	}
}

func TestSignUp_WithEmail(t *testing.T) {
	db := newMockAuthDB()
	srv := &authServer{db: db}

	resp, err := srv.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "pass123",
		Email:    "new@example.com",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
	if resp.User == nil {
		t.Fatal("expected non-nil user")
	}
	if resp.User.Email != "new@example.com" {
		t.Errorf("expected email=new@example.com, got %s", resp.User.Email)
	}
}

// ===== Benchmarks =====

func BenchmarkSignIn(b *testing.B) {
	db := newMockAuthDB()
	hash, _ := HashPassword("pass123")
	db.addUser("benchuser", hash, "bench@example.com")
	srv := &authServer{db: db}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.SignIn(ctx, &gen.SignInRequest{
			Username: "benchuser",
			Password: "pass123",
		})
	}
}

func BenchmarkSignUp(b *testing.B) {
	db := newMockAuthDB()
	srv := &authServer{db: db}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.SignUp(ctx, &gen.SignUpRequest{
			Username: fmt.Sprintf("user%d", i),
			Password: "pass123",
		})
	}
}
