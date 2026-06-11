package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// authServer implements gen.AuthServiceServer
type authServer struct {
	gen.UnimplementedAuthServiceServer
	db *DB
}

func newAuthServer(db *DB) *authServer {
	return &authServer{db: db}
}

func (a *authServer) SignIn(ctx context.Context, req *gen.SignInRequest) (*gen.AuthResponse, error) {
	username := req.GetUsername()
	password := req.GetPassword()

	if username == "" || password == "" {
		return &gen.AuthResponse{
			Success: false,
			Message: "username and password are required",
		}, nil
	}

	// Check if user exists
	exists, err := a.db.UserExists(username)
	if err != nil {
		log.Printf("SignIn: UserExists error for %s: %v", username, err)
		return &gen.AuthResponse{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if !exists {
		return &gen.AuthResponse{
			Success: false,
			Message: "user not found",
		}, nil
	}

	// Verify password
	storedHash, err := a.db.GetUserPasswordHash(username)
	if err != nil {
		log.Printf("SignIn: GetUserPasswordHash error for %s: %v", username, err)
		return &gen.AuthResponse{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if !CheckPassword(password, storedHash) {
		return &gen.AuthResponse{
			Success: false,
			Message: "invalid password",
		}, nil
	}

	// Get user data
	userID, _ := a.db.GetUserIdByUsername(username)
	avatarURL, _ := a.db.GetUserAvatar(username)
	email := ""
	bio := ""
	status := ""
	var createdAt time.Time
	var lastSeenAt time.Time

	// Fetch extended profile
	row := a.db.QueryRow(
		"SELECT COALESCE(email, ''), COALESCE(bio, ''), COALESCE(status, ''), created_at, last_seen_at FROM users WHERE username=$1",
		username,
	)
	_ = row.Scan(&email, &bio, &status, &createdAt, &lastSeenAt)

	// Generate session token (UUID-based)
	token := uuid.New().String()

	// Update last seen
	_ = a.db.UpdateLastSeen(username)

	log.Printf("SignIn: %s (ID: %s)", username, userID)

	return &gen.AuthResponse{
		Success: true,
		Token:   token,
		Message: "sign in successful",
		User: &gen.User{
			Id:            userID,
			Username:      username,
			Email:         email,
			AvatarUrl:     avatarURL,
			Bio:           bio,
			Status:        status,
			CreatedAt:     timestamppb.New(createdAt),
			LastSeenAt:    timestamppb.New(lastSeenAt),
		},
	}, nil
}

func (a *authServer) SignUp(ctx context.Context, req *gen.SignUpRequest) (*gen.AuthResponse, error) {
	username := req.GetUsername()
	password := req.GetPassword()
	email := req.GetEmail()

	if username == "" || password == "" {
		return &gen.AuthResponse{
			Success: false,
			Message: "username and password are required",
		}, nil
	}

	// Check if user already exists
	exists, err := a.db.UserExists(username)
	if err != nil {
		log.Printf("SignUp: UserExists error for %s: %v", username, err)
		return &gen.AuthResponse{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if exists {
		return &gen.AuthResponse{
			Success: false,
			Message: "username already taken",
		}, nil
	}

	// Check email uniqueness if provided
	if email != "" {
		emailExists, err := a.db.EmailExists(email)
		if err != nil {
			log.Printf("SignUp: EmailExists error for %s: %v", email, err)
			return &gen.AuthResponse{
				Success: false,
				Message: "internal error",
			}, nil
		}
		if emailExists {
			return &gen.AuthResponse{
				Success: false,
				Message: "email already in use",
			}, nil
		}
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		log.Printf("SignUp: HashPassword error for %s: %v", username, err)
		return &gen.AuthResponse{
			Success: false,
			Message: "internal error",
		}, nil
	}

	// Save user
	err = a.db.SaveUserWithEmail(username, passwordHash, email)
	if err != nil {
		log.Printf("SignUp: SaveUserWithEmail error for %s: %v", username, err)
		return &gen.AuthResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create user: %v", err),
		}, nil
	}

	// Get created user data
	userID, _ := a.db.GetUserIdByUsername(username)
	now := time.Now()

	// Generate session token
	token := uuid.New().String()

	log.Printf("SignUp: new user %s (ID: %s)", username, userID)

	return &gen.AuthResponse{
		Success: true,
		Token:   token,
		Message: "sign up successful",
		User: &gen.User{
			Id:         userID,
			Username:   username,
			Email:      email,
			CreatedAt:  timestamppb.New(now),
			LastSeenAt: timestamppb.New(now),
		},
	}, nil
}
