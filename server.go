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
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	firebase "firebase.google.com/go/v4"
)

const ServerVersion = "1.1.2.11"

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
	aiChatManager *AIChatManager
}

func (s *server) logErrorOnce(key string, format string, v ...interface{}) {
	now := time.Now()
	if last, ok := s.recentErrors.Load(key); ok {
		if now.Sub(last.(time.Time)) < 30*time.Second {
			return
		}
	}
	s.recentErrors.Store(key, now)
	log.Printf(format, v...)
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
	log.Printf("[FCM %s] %s", level, msg)
}

// resolveUserId converts a potential username to a user ID if needed
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
