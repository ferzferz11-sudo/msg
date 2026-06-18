package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *server) getAIChatManager() *AIChatManager {
	if s.aiChatManager == nil {
		s.aiChatManager = NewAIChatManager(s.db.DB)
	}
	return s.aiChatManager
}

func (s *server) ChatWithAI(req *gen.AIChatRequest, stream gen.ChatService_ChatWithAIServer) error {
	userID := GetUserID(stream.Context())
	if userID == "" {
		return status.Error(codes.Unauthenticated, "unauthorized")
	}

	sessionID := req.SessionId
	manager := s.getAIChatManager()

	// If no session_id, create a new session
	if sessionID == "" {
		agentType := "owl"
		if req.AgentId != "" {
			agentType = "hermes"
		}
		var err error
		sessionID, err = manager.CreateSession(userID, agentType)
		if err != nil {
			return status.Error(codes.Internal, "failed to create session")
		}

		// Also create in chats table for UI list
		username := userID
		if uname, err := s.db.GetUsernameByID(userID); err == nil && uname != "" {
			username = uname
		}
		participantsJSON, _ := json.Marshal([]string{userID})
		chatName := "🤖 AI Чат"
		if agentType == "hermes" {
			chatName = "🤖 Hermes"
		}
		_, _ = s.db.Exec(
			"INSERT INTO chats (id, name, type, participants, creator_username, creator_id) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING",
			sessionID, chatName, agentType, string(participantsJSON), username, userID,
		)
	}

	// Get session info
	session, err := manager.GetSession(sessionID)
	if err != nil {
		return status.Error(codes.NotFound, "session not found")
	}

	// Verify ownership
	if session.UserID != userID {
		return status.Error(codes.PermissionDenied, "not your session")
	}

	// Rate limit check
	settings, _ := manager.GetSettings(sessionID)
	hasCustomKey := settings.UserAPIKey != ""
	if hasCustomKey {
		if !owlRateLimiter.allow(userID) {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded: max 10 requests per minute")
		}
	} else {
		if !freeTierRateLimiter.allow(userID) {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded: max 20 requests per hour")
		}
	}

	// Save user message
	manager.AddMessage(sessionID, "user", req.Message, "")

	if session.AgentType == "hermes" {
		// Route to Hermes Orchestrator
		logger.Infof("[ChatWithAI] routing to Hermes orchestrator: session=%s user=%s", sessionID, userID)

		// Update active agent if specified
		if req.AgentId != "" {
			manager.UpdateSession(sessionID, "", "", req.AgentId, "")
		}

		var fullResponse strings.Builder
		err = s.hermesOrchestrator.Orchestrate(stream.Context(), userID, sessionID, req.Message,
			func(token string, finished bool) error {
				if !finished && token != "" {
					fullResponse.WriteString(token)
				}
				return stream.Send(&gen.AIChatResponse{
					Token:    token,
					Finished: finished,
				})
			})
		if err != nil {
			logger.Errorf("[ChatWithAI] orchestrator error: %v", err)
			// Refund rate limit slot on failure
			if hasCustomKey {
				owlRateLimiter.cancel(userID)
			} else {
				freeTierRateLimiter.cancel(userID)
			}
			stream.Send(&gen.AIChatResponse{Finished: true, Error: err.Error()})
		}

		// Save assistant response
		assistantResponse := fullResponse.String()
		if assistantResponse != "" {
			manager.AddMessage(sessionID, "assistant", assistantResponse, "")
		}
	} else {
		// Route to OWL (OpenRouter)
		logger.Infof("[ChatWithAI] routing to OWL: session=%s user=%s", sessionID, userID)

		// Build history
		history, _ := manager.GetHistory(sessionID, 50)

		// Convert history to OpenRouter format
		orHistory := make([]map[string]string, 0, len(history))
		for _, h := range history {
			orHistory = append(orHistory, map[string]string{"role": h.Role, "content": h.Content})
		}

		// System prompt
		systemPrompt := session.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = `Вы — AI-ассистент OWL в мессенджере Lavender. Отвечайте кратко и по делу на русском языке.
Вы можете помочь с вопросами о приложении Lavender, настройками, темами, функциями.
Будьте дружелюбны, но лаконичны. Не выдавайте себя за человека.`
		}

		// Model and API key
		model := settings.ModelOverride
		if model == "" {
			model = session.Model
		}
		if model == "" {
			model = s.owlModel
		}
		apiKey := settings.UserAPIKey
		if apiKey == "" {
			apiKey = s.owlApiKey
		}

		// Stream via OpenRouter
		err = streamOpenRouter(stream.Context(), apiKey, model, systemPrompt, orHistory,
			func(token string, finished bool) error {
				return stream.Send(&gen.AIChatResponse{
					Token:    token,
					Finished: finished,
				})
			})
		if err != nil {
			logger.Errorf("[ChatWithAI] OpenRouter error: %v", err)
			// Refund rate limit slot on failure
			if hasCustomKey {
				owlRateLimiter.cancel(userID)
			} else {
				freeTierRateLimiter.cancel(userID)
			}
			stream.Send(&gen.AIChatResponse{Finished: true, Error: err.Error()})
		}

		// Save assistant response (collect from stream)
		// For simplicity, we save after streaming completes
		// TODO: collect tokens if needed for DB saving
	}

	// Update chat last message
	_, _ = s.db.Exec("UPDATE chats SET last_message_text=$1, last_message_time=NOW() WHERE id=$2",
		truncateString(req.Message, 100), sessionID)

	return nil
}

func (s *server) GetAIChatHistory(ctx context.Context, req *gen.GetAIChatHistoryRequest) (*gen.GetAIChatHistoryResponse, error) {
	if req.SessionId == "" {
		return &gen.GetAIChatHistoryResponse{}, nil
	}
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetAIChatHistoryResponse{}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.GetAIChatHistoryResponse{}, nil
	}
	if session.UserID != userID {
		return &gen.GetAIChatHistoryResponse{}, nil
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	messages, err := manager.GetHistory(req.SessionId, limit)
	if err != nil {
		return &gen.GetAIChatHistoryResponse{}, nil
	}

	pbMessages := make([]*gen.AIChatMessage, 0, len(messages))
	for _, m := range messages {
		pbMessages = append(pbMessages, &gen.AIChatMessage{
			Role:      m.Role,
			Content:   m.Content,
			AgentId:   m.AgentID,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return &gen.GetAIChatHistoryResponse{Messages: pbMessages}, nil
}

func (s *server) GetAIChatSettings(ctx context.Context, req *gen.GetAIChatSettingsRequest) (*gen.AIChatSettings, error) {
	if req.SessionId == "" {
		return &gen.AIChatSettings{}, nil
	}
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.AIChatSettings{}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.AIChatSettings{}, nil
	}
	if session.UserID != userID {
		return &gen.AIChatSettings{}, nil
	}

	settings, _ := manager.GetSettings(req.SessionId)

	// Calculate remaining requests
	remaining := int32(freeTierRateLimiter.remaining(userID))
	limit := int32(20)
	windowSec := int32(3600)
	if settings.UserAPIKey != "" {
		remaining = int32(owlRateLimiter.remaining(userID))
		limit = 10
		windowSec = 60
	}

	return &gen.AIChatSettings{
		SessionId:        req.SessionId,
		UserApiKey:       settings.UserAPIKey,
		Model:            settings.ModelOverride,
		IsUsingCustomKey: settings.UserAPIKey != "",
		Remaining:        remaining,
		Limit:            limit,
		WindowSeconds:    windowSec,
	}, nil
}

func (s *server) UpdateAIChatSettings(ctx context.Context, req *gen.UpdateAIChatSettingsRequest) (*gen.UpdateAIChatSettingsResponse, error) {
	if req.SessionId == "" {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "session_id required"}, nil
	}
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "unauthorized"}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "session not found"}, nil
	}
	if session.UserID != userID {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "not your session"}, nil
	}

	err = manager.SaveSettings(req.SessionId, req.ApiKey, req.Model)
	if err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: err.Error()}, nil
	}

	return &gen.UpdateAIChatSettingsResponse{Success: true, Message: "OK"}, nil
}

func (s *server) GetAIChats(ctx context.Context, req *gen.GetAIChatsRequest) (*gen.GetAIChatsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetAIChatsResponse{}, nil
	}

	var chats []*gen.AIChatInfo

	// OWL chats from chats table
	owlRows, err := s.db.Query(
		"SELECT id, name, type, created_at FROM chats WHERE type = 'owl' AND creator_id = $1 ORDER BY created_at ASC",
		userID,
	)
	if err == nil {
		for owlRows.Next() {
			var id, name, chatType string
			var createdAt time.Time
			if err := owlRows.Scan(&id, &name, &chatType, &createdAt); err == nil {
				owlSet := owlSessions.getSettings(id)
				chats = append(chats, &gen.AIChatInfo{
					Id:               id,
					Name:             name,
					Type:             chatType,
					CreatedAt:        createdAt.Format(time.RFC3339),
					IsUsingCustomKey: owlSet.UserAPIKey != "",
					Model:            owlSet.Model,
				})
			}
		}
		owlRows.Close()
	}

	// Hermes chats from chats table (unified with OWL)
	hermesRows, err := s.db.Query(
		"SELECT id, name, type, created_at FROM chats WHERE type = 'hermes' AND creator_id = $1 ORDER BY created_at ASC",
		userID,
	)
	if err == nil {
		for hermesRows.Next() {
			var id, name, chatType string
			var createdAt time.Time
			if err := hermesRows.Scan(&id, &name, &chatType, &createdAt); err == nil {
				hermSet := hermesSettings.getSettings(id)
				chats = append(chats, &gen.AIChatInfo{
					Id:               id,
					Name:             name,
					Type:             chatType,
					CreatedAt:        createdAt.Format(time.RFC3339),
					IsUsingCustomKey: hermSet.UserAPIKey != "",
					Model:            hermSet.Model,
				})
			}
		}
		hermesRows.Close()
	}

	return &gen.GetAIChatsResponse{Chats: chats}, nil
}

func (s *server) RenameAIChat(ctx context.Context, req *gen.RenameAIChatRequest) (*gen.RenameAIChatResponse, error) {
	if req.ChatId == "" || req.NewName == "" {
		return &gen.RenameAIChatResponse{Success: false, Error: "chat_id and new_name required"}, nil
	}
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.RenameAIChatResponse{Success: false, Error: "unauthorized"}, nil
	}

	// Try OWL chat first
	var creatorID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&creatorID)
	if err == nil && creatorID == userID {
		_, err = s.db.Exec("UPDATE chats SET name = $1 WHERE id = $2", req.NewName, req.ChatId)
		if err != nil {
			return &gen.RenameAIChatResponse{Success: false, Error: err.Error()}, nil
		}
		return &gen.RenameAIChatResponse{Success: true}, nil
	}

	// Try Hermes session
	var hermesUserID string
	err = s.db.QueryRow("SELECT user_id FROM hermes_sessions WHERE id = $1", req.ChatId).Scan(&hermesUserID)
	if err == nil && hermesUserID == userID {
		_, err = s.db.Exec("UPDATE hermes_sessions SET name = $1 WHERE id = $2", req.NewName, req.ChatId)
		if err != nil {
			return &gen.RenameAIChatResponse{Success: false, Error: err.Error()}, nil
		}
		// Also update in chats if exists
		_, _ = s.db.Exec("UPDATE chats SET name = $1 WHERE id = $2", req.NewName, req.ChatId)
		return &gen.RenameAIChatResponse{Success: true}, nil
	}

	return &gen.RenameAIChatResponse{Success: false, Error: "chat not found or not your chat"}, nil
}

func (s *server) GetFreeModels(_ context.Context, _ *gen.GetFreeModelsRequest) (*gen.GetFreeModelsResponse, error) {
	models, err := s.db.GetFreeModels()
	if err != nil {
		logger.Errorf("[FreeModels] GetFreeModels error: %v", err)
		return &gen.GetFreeModelsResponse{}, nil
	}
	result := make([]*gen.FreeModelInfo, 0, len(models))
	for _, m := range models {
		result = append(result, &gen.FreeModelInfo{
			ModelId:     m.ModelID,
			DisplayName: m.DisplayName,
			SortOrder:   int32(m.SortOrder),
		})
	}
	return &gen.GetFreeModelsResponse{Models: result}, nil
}

func (s *server) SetFreeModel(ctx context.Context, req *gen.SetFreeModelRequest) (*gen.SetFreeModelResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.SetFreeModelResponse{Success: false, Error: "unauthorized"}, nil
	}
	if !s.db.IsSuperAdmin(userID) {
		return &gen.SetFreeModelResponse{Success: false, Error: "admin only"}, nil
	}
	if req.ModelId == "" {
		return &gen.SetFreeModelResponse{Success: false, Error: "model_id required"}, nil
	}
	if err := s.db.AddFreeModel(req.ModelId, req.DisplayName, int(req.SortOrder)); err != nil {
		return &gen.SetFreeModelResponse{Success: false, Error: err.Error()}, nil
	}
	return &gen.SetFreeModelResponse{Success: true}, nil
}

func (s *server) RemoveFreeModel(ctx context.Context, req *gen.RemoveFreeModelRequest) (*gen.RemoveFreeModelResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.RemoveFreeModelResponse{Success: false, Error: "unauthorized"}, nil
	}
	if !s.db.IsSuperAdmin(userID) {
		return &gen.RemoveFreeModelResponse{Success: false, Error: "admin only"}, nil
	}
	if err := s.db.RemoveFreeModel(req.ModelId); err != nil {
		return &gen.RemoveFreeModelResponse{Success: false, Error: err.Error()}, nil
	}
	return &gen.RemoveFreeModelResponse{Success: true}, nil
}

// generateChatName creates a localized name with sequential number for a new AI chat.
func generateChatName(chatType string, number int) string {
	switch chatType {
	case "owl":
		return fmt.Sprintf("Лава ИИ #%d", number)
	case "hermes":
		return fmt.Sprintf("Оркестратор #%d", number)
	default:
		return fmt.Sprintf("AI Chat #%d", number)
	}
}

// truncateString truncates a string to maxLen and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
