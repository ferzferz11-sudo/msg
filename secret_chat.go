package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"LavenderMessenger/gen"
	"github.com/google/uuid"
	"google.golang.org/grpc/peer"
)

// getCallerUsernameSecret получает имя вызывающего клиента из хаба для secret chat RPC
func (s *server) getCallerUsernameSecret(ctx context.Context) string {
	var callerUsername string
	if _, ok := peer.FromContext(ctx); ok {
		s.hub.mu.RLock()
		for _, name := range s.hub.clients {
			if name != "Anonymous" && name != "" {
				callerUsername = name
				break
			}
		}
		s.hub.mu.RUnlock()
	}
	return callerUsername
}

// CreateSecretChat создает новый секретный чат (E2EE) между двумя пользователями
func (s *server) CreateSecretChat(ctx context.Context, req *gen.CreateSecretChatRequest) (*gen.CreateSecretChatResponse, error) {
	targetUser := req.TargetUsername
	if req.TargetUserId != "" {
		if resolved := s.resolveUsername(req.TargetUserId); resolved != "" {
			targetUser = resolved
		}
	}

	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.CreateSecretChatResponse{Success: false, Message: "Not authenticated"}, fmt.Errorf("not authenticated")
	}

	// Version check: secret chats require client >= 1.0.7.1
	if req.ClientVersion != "" && compareVersions(req.ClientVersion, "1.0.7.1") < 0 {
		log.Printf("Secret chat rejected: client %s is too old (need >= 1.0.7.1)", req.ClientVersion)
		return &gen.CreateSecretChatResponse{Success: false, Message: "Secret chats require client version 1.0.7.1 or higher"}, fmt.Errorf("client version too old")
	}

	chatID := "secret_" + uuid.New().String()
	participants := []string{callerUsername, targetUser}
	err := s.db.CreateSecretChat(chatID, callerUsername+" 🔒 "+targetUser, callerUsername, participants)
	if err != nil {
		log.Printf("Failed to create secret chat: %v", err)
		return &gen.CreateSecretChatResponse{Success: false, Message: "Failed to create secret chat"}, err
	}

	userId, _ := s.db.GetUserIdByUsername(callerUsername)
	if req.PublicKey != "" && userId != "" {
		_ = s.db.StoreSecretChatKey(chatID, userId, req.PublicKey)
	}

	log.Printf("Secret chat created: %s (creator: %s, peer: %s)", chatID, callerUsername, targetUser)
	return &gen.CreateSecretChatResponse{
		ChatId:  chatID,
		Success: true,
		Message: "Secret chat created",
	}, nil
}

// ExchangeSecretKey обменивается публичными ключами для E2EE
func (s *server) ExchangeSecretKey(ctx context.Context, req *gen.ExchangeSecretKeyRequest) (*gen.ExchangeSecretKeyResponse, error) {
	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.ExchangeSecretKeyResponse{Success: false}, fmt.Errorf("not authenticated")
	}

	userId, _ := s.db.GetUserIdByUsername(callerUsername)
	if req.PublicKey != "" && userId != "" {
		if err := s.db.StoreSecretChatKey(req.ChatId, userId, req.PublicKey); err != nil {
			log.Printf("Failed to store secret chat key: %v", err)
			return &gen.ExchangeSecretKeyResponse{Success: false}, err
		}
	}

	keys, err := s.db.GetAllSecretChatKeys(req.ChatId)
	if err != nil {
		return &gen.ExchangeSecretKeyResponse{Success: true, PeerHasKey: false}, nil
	}

	var peerKey string
	found := false
	for uid, key := range keys {
		if uid != userId {
			peerKey = key
			found = true
			break
		}
	}

	if found && len(keys) >= 2 {
		_ = s.db.SetSecretChatE2EEReady(req.ChatId, true)
		log.Printf("E2EE ready for secret chat: %s", req.ChatId)
	}

	log.Printf("Secret key exchanged for chat: %s (user: %s, peer_found: %v)", req.ChatId, callerUsername, found)
	return &gen.ExchangeSecretKeyResponse{
		Success:       true,
		PeerPublicKey: peerKey,
		PeerHasKey:    found,
	}, nil
}

// GetSecretChatKey получает публичный ключа партнера для E2EE
func (s *server) GetSecretChatKey(ctx context.Context, req *gen.GetSecretChatKeyRequest) (*gen.GetSecretChatKeyResponse, error) {
	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.GetSecretChatKeyResponse{PeerHasKey: false}, fmt.Errorf("not authenticated")
	}

	userId, _ := s.db.GetUserIdByUsername(callerUsername)
	keys, err := s.db.GetAllSecretChatKeys(req.ChatId)
	if err != nil {
		return &gen.GetSecretChatKeyResponse{PeerHasKey: false}, nil
	}

	var peerKey string
	found := false
	for uid, key := range keys {
		if uid != userId {
			peerKey = key
			found = true
			break
		}
	}

	return &gen.GetSecretChatKeyResponse{
		PeerPublicKey: peerKey,
		PeerHasKey:    found,
	}, nil
}

// compareVersions compares two semantic version strings (e.g. "1.0.7.1" vs "1.0.6.34")
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}
