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
//   server_profile.go   — [DEPRECATED] Legacy profile: UpdateUsername, UpdatePassword, AdminUpdatePassword, MarkRead, UpdateAvatar, DeleteProfile
//   server_profile_v2.go — ProfileService v2: GetProfile, UpdateProfile, UpdateAvatar, DeleteProfile, GetUserSettings, UpdateUserSettings
//   server_push.go      — RegisterToken, sendPushNotification, broadcastOnlineUsers, etc.
//   server_contacts.go  — AddContact, RemoveContact, GetContacts, GetChatListVersion
//   server_themes.go    — GetThemes, SaveTheme, SetCurrentTheme, DeleteTheme
//   server_drafts.go    — GetFCMLogs, SaveDraft, GetDraft, DeleteDraft
//   server_muted.go     — GetMutedChats, SetMutedChat
//   server_favorites.go — GetUserId, AddFavorite, RemoveFavorite, GetFavorites
//   server_ai_v2.go     — AI Services v2: ChatWithAIV2, Agent CRUD, ListTools

package main

import (
	"LavenderMessenger/gen"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/google/uuid"
)

const ServerVersion = "1.3.0.16"

// Service versions for client capability negotiation.
// Service versions for client capability negotiation.
const (
	AuthServiceVersion = "2.0" // AuthService v2 (JWT) — current
	ChatServiceVersion = "2.0" // ChatService v2: Bearer token in Chat stream + Pin/Mute/Search/Read
	AIServiceVersion   = "2.0"
	FileServiceVersion = "1.0"
	PushServiceVersion = "1.0"
)

// ProfileServiceVersion is set in main() — "2.0" on dev, "1.0" on prod.
var ProfileServiceVersion = "1.0"

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
	owlModel     string // Default OWL model
	owlApiKey    string // Default OpenRouter API key

	// Hermes DB (for hermes-agent service)
	hermesDB *HermesDB

	// Remote Agent Manager
	remoteAgentManager *RemoteAgentManager

	// AI Gateway v2
	aiGateway *AIGateway

	// Shutdown state
	isShuttingDown atomic.Bool
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
