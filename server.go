// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.
//
// Refactored in v1.1.2.9: methods split into separate files by domain:
//   server_chat.go      — Chat, Typing, CallSession, GetClients
//   server_users.go     — GetAllUsers, UpdateProfile, GetUserProfile, GetUserAvatar
//   server_chats.go     — GetAllChats, GetChats, CreateDirectChat, CreateGroupChat, DeleteChat, etc.
//   server_messages.go  — GetHistory, SetReaction, DeleteMessages, EditMessage
//   server_profile.go   — UpdateUsername, UpdatePassword, AdminUpdatePassword, MarkRead, UpdateAvatar, DeleteProfile
//   server_push.go      — RegisterToken, sendPushNotification, broadcastOnlineUsers, etc.
//   server_contacts.go  — AddContact, RemoveContact, GetContacts, GetChatListVersion
//   server_themes.go    — GetThemes, SaveTheme, SetCurrentTheme, DeleteTheme
//   server_drafts.go    — GetFCMLogs, SaveDraft, GetDraft, DeleteDraft
//   server_muted.go     — GetMutedChats, SetMutedChat
//   server_favorites.go — GetUserId, AddFavorite, RemoveFavorite, GetFavorites
//   server_ai.go        — ChatWithOWL, ChatWithAI, ChatWithOrchestrator, Hermes sessions, etc.

package main

import (
	"LavenderMessenger/gen"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	firebase "firebase.google.com/go/v4"
)

const ServerVersion = "1.2.0.10"

// Service versions for client capability negotiation
const (
	AuthServiceVersion    = "2.0" // AuthService v2 (JWT) — current
	ChatServiceVersion    = "2.0" // ChatService v2: Bearer token in Chat stream + Pin/Mute/Search/Read
	ProfileServiceVersion = "2.0" // ProfileService v2 (JWT) — dev only, prod uses v1 via ChatService
	AIServiceVersion      = "1.0"
	FileServiceVersion    = "1.0"
	PushServiceVersion    = "1.0"
)

// server implements the gRPC ChatService interface
type server struct {
	gen.UnimplementedChatServiceServer
	hub          *Hub          // Hub for managing client connections
	db           *DB           // Database for message persistence
	firebaseApp  *firebase.App // Firebase Admin SDK instance
	recentMsgs   sync.Map      // Cache for deduplicating identical rapid messages
	recentErrors sync.Map      // map[string]time.Time to prevent duplicate error logs
	fcmLogs      []*gen.FCMLogEntry
	fcmLogsMu    sync.Mutex
	owlModel     string        // Default OWL model
	owlApiKey    string        // Default OpenRouter API key

	// Hermes Orchestrator
	hermesOrchestrator *Orchestrator
	hermesDB           *HermesDB

	// AI Chat Manager (unified for OWL + Hermes)
	aiChatManager     *AIChatManager
	aiChatManagerOnce sync.Once
}

func (s *server) logErrorOnce(key string, format string, v ...interface{}) {
	now := time.Now()
	if last, ok := s.recentErrors.Load(key); ok {
		if now.Sub(last.(time.Time)) < 30*time.Second {
			return
		}
	}
	s.recentErrors.Store(key, now)
	logger.Errorf(format, v...)
}

func (s *server) logFCM(level, format string, v ...interface{}) {
	s.fcmLogsMu.Lock()
	defer s.fcmLogsMu.Unlock()

	msg := fmt.Sprintf(format, v...)
	entry := &gen.FCMLogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   msg,
	}
	s.fcmLogs = append(s.fcmLogs, entry)
	if len(s.fcmLogs) > 100 {
		s.fcmLogs = s.fcmLogs[1:]
	}
	logger.Infof("[FCM %s] %s", level, msg)
}

// isUUID checks if a string is a valid UUID
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// resolveDisplayName resolves a UUID or username to a username for display/logging
func resolveDisplayName(db *DB, identifier string) string {
	if identifier == "" {
		return ""
	}
	if isUUID(identifier) {
		username, err := db.GetUserByID(identifier)
		if err == nil && username != "" {
			return username
		}
		return identifier
	}
	return identifier
}

// resolveUserId converts a potential username to a user ID if needed
// Deprecated: v1 username→UUID fallback. Use UUID identifiers directly for v2-only handlers.
func (s *server) resolveUserId(identifier string) string {
	if identifier == "" {
		return ""
	}
	// Check if it's a UUID
	if _, err := uuid.Parse(identifier); err == nil {
		return identifier
	}
	// It's a username, try to get the ID
	id, err := s.db.GetUserIdByUsername(identifier)
	if err == nil && id != "" {
		return id
	}
	return identifier
}

// resolveUsername converts a potential user ID to a username if needed
// Deprecated: v1 UUID→username fallback. Use UUID identifiers directly for v2-only handlers.
func (s *server) resolveUsername(identifier string) string {
	if identifier == "" {
		return ""
	}
	var name string
	err := s.db.QueryRow("SELECT username FROM users WHERE id=$1::uuid", identifier).Scan(&name)
	if err != nil {
		return identifier
	}
	return name
}
