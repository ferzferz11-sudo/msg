package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"LavenderMessenger/gen"

	"github.com/google/uuid"
)

func (s *server) getCallerUsernameSecret(ctx context.Context) string {
	userID := GetUserID(ctx)
	if userID == "" {
		return ""
	}
	username, _ := s.db.GetUsernameByID(userID)
	if username != "" {
		return username
	}
	return userID
}

func (s *server) CreateSecretChat(ctx context.Context, req *gen.CreateSecretChatRequest) (*gen.CreateSecretChatResponse, error) {
	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.CreateSecretChatResponse{Success: false, Message: "Not authenticated"}, fmt.Errorf("not authenticated")
	}

	targetUser := req.TargetUsername
	if req.TargetUserId != "" {
		if resolved := resolveDisplayName(s.db, req.TargetUserId); resolved != "" {
			targetUser = resolved
		}
	}

	if req.ClientVersion != "" && compareVersions(req.ClientVersion, "1.0.7.1") < 0 {
		return &gen.CreateSecretChatResponse{Success: false, Message: "Secret chats require client version 1.0.7.1 or higher"}, fmt.Errorf("client version too old")
	}

	chatID := "secret_" + uuid.New().String()
	participants := []string{callerUsername, targetUser}
	err := s.db.CreateSecretChat(chatID, callerUsername+" 🔒 "+targetUser, callerUsername, participants)
	if err != nil {
		return &gen.CreateSecretChatResponse{Success: false, Message: "Failed to create secret chat"}, err
	}

	userId, _ := s.db.GetUserIdByUsername(callerUsername)
	if req.PublicKey != "" && userId != "" {
		_ = s.db.StoreSecretChatKey(chatID, userId, req.PublicKey)
	}

	return &gen.CreateSecretChatResponse{
		ChatId:  chatID,
		Success: true,
		Message: "Secret chat created",
	}, nil
}

func (s *server) ExchangeSecretKey(ctx context.Context, req *gen.ExchangeSecretKeyRequest) (*gen.ExchangeSecretKeyResponse, error) {
	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.ExchangeSecretKeyResponse{Success: false}, fmt.Errorf("not authenticated")
	}

	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		return &gen.ExchangeSecretKeyResponse{Success: false}, fmt.Errorf("chat not found")
	}
	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err == nil {
		isParticipant := false
		for _, p := range participants {
			if p == callerUsername {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			return &gen.ExchangeSecretKeyResponse{Success: false}, fmt.Errorf("not a participant")
		}
	}

	userId, _ := s.db.GetUserIdByUsername(callerUsername)
	if req.PublicKey != "" && userId != "" {
		if err := s.db.StoreSecretChatKey(req.ChatId, userId, req.PublicKey); err != nil {
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
	}

	return &gen.ExchangeSecretKeyResponse{
		Success:       true,
		PeerPublicKey: peerKey,
		PeerHasKey:    found,
	}, nil
}

func (s *server) GetSecretChatKey(ctx context.Context, req *gen.GetSecretChatKeyRequest) (*gen.GetSecretChatKeyResponse, error) {
	callerUsername := s.getCallerUsernameSecret(ctx)
	if callerUsername == "" {
		return &gen.GetSecretChatKeyResponse{PeerHasKey: false}, fmt.Errorf("not authenticated")
	}

	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		return &gen.GetSecretChatKeyResponse{PeerHasKey: false}, fmt.Errorf("chat not found")
	}
	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err == nil {
		isParticipant := false
		for _, p := range participants {
			if p == callerUsername {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			return &gen.GetSecretChatKeyResponse{PeerHasKey: false}, fmt.Errorf("not a participant")
		}
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
