package main

import (
	"LavenderMessenger/gen"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ======= Mock DB for auth tests =======

type mockAuthDB struct {
	mu     sync.Mutex
	users  map[string]*mockUser // key: username
	emails map[string]bool      // key: email
	nextID int
}

type mockUser struct {
	id           string
	username     string
	passwordHash string
	email        string
	avatarURL    string
	bio          string
	status       string
	createdAt    time.Time
	lastSeenAt   time.Time
}

func newMockAuthDB() *mockAuthDB {
	return &mockAuthDB{
		users:  make(map[string]*mockUser),
		emails: make(map[string]bool),
		nextID: 1,
	}
}

func (m *mockAuthDB) UserExists(user string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.users[user]
	return ok, nil
}

func (m *mockAuthDB) EmailExists(email string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.emails[email], nil
}

func (m *mockAuthDB) GetUserPasswordHash(user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[user]
	if !ok {
		return "", errors.New("user not found")
	}
	return u.passwordHash, nil
}

func (m *mockAuthDB) SaveUserWithEmail(user, hash, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user] = &mockUser{
		id:           fmt.Sprintf("user-%d", m.nextID),
		username:     user,
		passwordHash: hash,
		email:        email,
		createdAt:    time.Now(),
		lastSeenAt:   time.Now(),
	}
	m.nextID++
	if email != "" {
		m.emails[email] = true
	}
	return nil
}

func (m *mockAuthDB) GetUserIdByUsername(user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[user]
	if !ok {
		return "", errors.New("not found")
	}
	return u.id, nil
}

func (m *mockAuthDB) GetUserAvatar(user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[user]
	if !ok {
		return "", errors.New("not found")
	}
	return u.avatarURL, nil
}

func (m *mockAuthDB) UpdateLastSeen(user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[user]
	if !ok {
		return errors.New("not found")
	}
	u.lastSeenAt = time.Now()
	return nil
}

func (m *mockAuthDB) queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		err = errors.New("not found")
		return
	}
	return u.email, u.bio, u.status, u.createdAt, u.lastSeenAt, nil
}

func (m *mockAuthDB) addUser(username, password, email string) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	m.users[username] = &mockUser{
		id:           fmt.Sprintf("user-%d", m.nextID),
		username:     username,
		passwordHash: string(hash),
		email:        email,
		createdAt:    time.Now(),
		lastSeenAt:   time.Now(),
	}
	m.nextID++
	if email != "" {
		m.emails[email] = true
	}
}

// ======= SignIn Tests =======

func TestSignIn_Success(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	db.addUser("testuser", "password123", "test@example.com")

	s := &authServer{db: db}
	resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if !resp.Success {
		t.Fatal("SignIn should succeed")
	}
	if resp.Token == "" {
		t.Fatal("SignIn should return a token")
	}
	if resp.User == nil {
		t.Fatal("SignIn should return user data")
	}
	if resp.User.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", resp.User.Username)
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", resp.User.Email)
	}
}

func TestSignIn_WrongPassword(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	db.addUser("testuser", "password123", "")

	s := &authServer{db: db}
	resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "wrongpassword",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignIn should fail with wrong password")
	}
	if !strings.Contains(resp.Message, "invalid password") {
		t.Errorf("expected 'invalid password' message, got '%s'", resp.Message)
	}
}

func TestSignIn_UserNotFound(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()

	s := &authServer{db: db}
	resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
		Username: "nonexistent",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignIn should fail for non-existent user")
	}
	if !strings.Contains(resp.Message, "user not found") {
		t.Errorf("expected 'user not found' message, got '%s'", resp.Message)
	}
}

func TestSignIn_EmptyUsername(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
		Username: "",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignIn should fail with empty username")
	}
	if !strings.Contains(resp.Message, "required") {
		t.Errorf("expected 'required' message, got '%s'", resp.Message)
	}
}

func TestSignIn_EmptyPassword(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
		Username: "testuser",
		Password: "",
	})

	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignIn should fail with empty password")
	}
	if !strings.Contains(resp.Message, "required") {
		t.Errorf("expected 'required' message, got '%s'", resp.Message)
	}
}

// ======= SignUp Tests =======

func TestSignUp_Success(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("SignUp should succeed, got message: %s", resp.Message)
	}
	if resp.Token == "" {
		t.Fatal("SignUp should return a token")
	}
	if resp.User == nil {
		t.Fatal("SignUp should return user data")
	}
	if resp.User.Username != "newuser" {
		t.Errorf("expected username 'newuser', got '%s'", resp.User.Username)
	}
}

func TestSignUp_WithEmail(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("SignUp with email should succeed, got: %s", resp.Message)
	}
	if resp.User.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got '%s'", resp.User.Email)
	}
}

func TestSignUp_DuplicateUsername(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	db.addUser("existing", "password123", "")

	s := &authServer{db: db}
	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "existing",
		Password: "password456",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignUp should fail for duplicate username")
	}
	if !strings.Contains(resp.Message, "already taken") {
		t.Errorf("expected 'already taken' message, got '%s'", resp.Message)
	}
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	db.addUser("user1", "password123", "dup@example.com")

	s := &authServer{db: db}
	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "user2",
		Password: "password456",
		Email:    "dup@example.com",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignUp should fail for duplicate email")
	}
	if !strings.Contains(resp.Message, "email already in use") {
		t.Errorf("expected 'email already in use' message, got '%s'", resp.Message)
	}
}

func TestSignUp_EmptyUsername(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignUp should fail with empty username")
	}
	if !strings.Contains(resp.Message, "required") {
		t.Errorf("expected 'required' message, got '%s'", resp.Message)
	}
}

func TestSignUp_EmptyPassword(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("SignUp should fail with empty password")
	}
	if !strings.Contains(resp.Message, "required") {
		t.Errorf("expected 'required' message, got '%s'", resp.Message)
	}
}

func TestSignUp_EmptyEmail(t *testing.T) {
	t.Parallel()
	db := newMockAuthDB()
	s := &authServer{db: db}

	// Empty email should be allowed (email is optional)
	resp, err := s.SignUp(context.Background(), &gen.SignUpRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "",
	})

	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("SignUp with empty email should succeed, got: %s", resp.Message)
	}
}

// ======= Benchmarks =======

func BenchmarkSignIn(b *testing.B) {
	db := newMockAuthDB()
	db.addUser("benchuser", "password123", "bench@example.com")
	s := &authServer{db: db}
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.SignIn(ctx, &gen.SignInRequest{
				Username: "benchuser",
				Password: "password123",
			})
		}
	})
}

func BenchmarkSignUp(b *testing.B) {
	db := newMockAuthDB()
	s := &authServer{db: db}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.SignUp(ctx, &gen.SignUpRequest{
			Username: fmt.Sprintf("user%d", i),
			Password: "password123",
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword("benchmarkpassword123")
	}
}
