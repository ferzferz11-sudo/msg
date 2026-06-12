package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *server) ChatWithOWL(req *gen.OWLRequest, stream gen.ChatService_ChatWithOWLServer) error {
	userID := req.UserId
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}

	chatID := req.SessionId
	if chatID == "" {
		// Fallback for old clients
		chatID = "owl-" + userID
	}

	// Auto-create OWL chat in DB if it doesn't exist (fixes FK constraint on owl_messages)
	var existingChat string
	err := s.db.QueryRow("SELECT id FROM chats WHERE id = $1", chatID).Scan(&existingChat)
	if err != nil {
		// Chat doesn't exist — create it
		username := userID
		if uname, err := s.db.GetUsernameByID(userID); err == nil && uname != "" {
			username = uname
		}
		participantsJSON, _ := json.Marshal([]string{userID})
		_, _ = s.db.Exec(
			"INSERT INTO chats (id, name, type, participants, creator_username, creator_id) VALUES ($1, $2, 'owl', $3, $4, $5) ON CONFLICT (id) DO NOTHING",
			chatID, "🤖 Чат с AI", string(participantsJSON), username, userID,
		)
	}

	// Rate limit check: custom key users get 10/min, free tier gets 20/hour
	settings := owlSessions.getSettings(chatID)
	hasCustomKey := settings.UserAPIKey != ""
	if hasCustomKey {
		if !owlRateLimiter.allow(userID) {
			return fmt.Errorf("rate limit exceeded: max 10 requests per minute")
		}
	} else {
		if !freeTierRateLimiter.allow(userID) {
			return fmt.Errorf("rate limit exceeded: max 20 requests per hour for free tier. Set your own API key in chat settings for unlimited access")
		}
	}

	// System prompt
	systemPrompt := `Вы — AI-ассистент OWL в мессенджере Lavender. Отвечайте кратко и по делу на русском языке.
Вы можете помочь с вопросами о приложении Lavender, настройками, темами, функциями.
Будьте дружелюбны, но лаконичны. Не выдавайте себя за человека.`

	// Add user message to history
	owlSessions.addMessage(chatID, "user", req.Message)

	// Build context from history
	history := owlSessions.getHistory(chatID)

	// Use model from request, then per-chat setting, then server default
	model := req.Model
	if model == "" {
		model = settings.Model
	}
	if model == "" {
		model = s.owlModel
	}

	// Use API key from request, then per-chat setting, then server default
	apiKey := req.ApiKey
	if apiKey == "" {
		apiKey = settings.UserAPIKey
	}
	if apiKey == "" {
		apiKey = s.owlApiKey
	}

	log.Printf("OWL: chat=%s, user=%s, msg=%q, history_len=%d, model=%s, custom_key=%t",
		chatID, userID, req.Message, len(history), model, req.ApiKey != "" || settings.UserAPIKey != "")

	// Call OpenRouter
	log.Printf("OWL: calling OpenRouter for chat %s, model=%s, history_len=%d", chatID, model, len(history))
	response, err := callOpenRouterContext(context.Background(), apiKey, model, systemPrompt, history)
	if err != nil {
		log.Printf("OWL: OpenRouter error for chat %s: %v", chatID, err)
		// Refund rate limit slot on failure
		if hasCustomKey {
			owlRateLimiter.cancel(userID)
		} else {
			freeTierRateLimiter.cancel(userID)
		}
		return fmt.Errorf("AI service error: %w", err)
	}
	log.Printf("OWL: OpenRouter response for chat %s: %q (len=%d)", chatID, response, len(response))

	// Add assistant response to history
	owlSessions.addMessage(chatID, "assistant", response)

	// Update chat last message
	_, _ = s.db.Exec("UPDATE chats SET last_message_text=$1, last_message_time=NOW() WHERE id=$2",
		truncateString(response, 100), chatID)
	log.Printf("OWL: updated last_message_text for chat %s: %q", chatID, truncateString(response, 100))

	// Stream response in chunks
	words := strings.Fields(response)
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}
		isLast := i == len(words)-1
		if err := stream.Send(&gen.OWLResponse{
			Text:     chunk,
			Finished: isLast,
		}); err != nil {
			log.Printf("OWL: stream send error for chat %s: %v", chatID, err)
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}

	return nil
}

func (s *server) getNextChatNumber(userID, chatType string) int {
	var maxNum sql.NullInt64
	// Extract number from name like "Лава ИИ #3" or "Lava AI #3" or "Оркестратор #2"
	// Pattern: " #<number>" at end of name
	err := s.db.QueryRow(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(name FROM '#(\d+)$') AS INTEGER)), 0)
		FROM chats
		WHERE creator_id = $1 AND type = $2
		AND name ~ '#\d+$'
	`, userID, chatType).Scan(&maxNum)
	if err != nil || !maxNum.Valid {
		return 1
	}
	return int(maxNum.Int64) + 1
}

func (s *server) CreateOwlChat(_ context.Context, req *gen.CreateOwlChatRequest) (*gen.CreateOwlChatResponse, error) {
	log.Printf("CreateOwlChat: called user_id=%q name=%q", req.UserId, req.Name)
	if req.UserId == "" {
		log.Printf("CreateOwlChat: ERROR empty user_id")
		return &gen.CreateOwlChatResponse{Success: false, Message: "user_id is required"}, nil
	}

	// Look up username from users table by user_id
	username := req.UserId
	if uname, err := s.db.GetUsernameByID(req.UserId); err == nil && uname != "" {
		username = uname
	}
	log.Printf("CreateOwlChat: resolved username=%q for user_id=%q", username, req.UserId)

	// Generate sequential number for this user's OWL chats
	num := s.getNextChatNumber(req.UserId, "owl")
	name := req.Name
	if name == "" {
		name = generateChatName("owl", num)
	}

	chatID := "owl-" + uuid.New().String()
	log.Printf("CreateOwlChat: creating chat %q name=%q for user %q", chatID, name, username)

	participantsJSON, _ := json.Marshal([]string{req.UserId})
	_, err := s.db.Exec(
		"INSERT INTO chats (id, name, type, participants, creator_username, creator_id) VALUES ($1, $2, 'owl', $3, $4, $5)",
		chatID, name, string(participantsJSON), username, req.UserId,
	)
	if err != nil {
		log.Printf("CreateOwlChat: DB error: %v", err)
		return &gen.CreateOwlChatResponse{Success: false, Message: "failed to create chat: " + err.Error()}, nil
	}

	log.Printf("CreateOwlChat: SUCCESS chat_id=%q name=%q user=%q", chatID, name, username)
	return &gen.CreateOwlChatResponse{
		ChatId:  chatID,
		Name:    name,
		Success: true,
		Message: "OK",
	}, nil
}

func (s *server) DeleteOwlChat(_ context.Context, req *gen.DeleteOwlChatRequest) (*gen.DeleteOwlChatResponse, error) {
	if req.ChatId == "" {
		return &gen.DeleteOwlChatResponse{Success: false, Message: "chat_id is required"}, nil
	}
	if req.UserId == "" {
		return &gen.DeleteOwlChatResponse{Success: false, Message: "user_id is required"}, nil
	}

	var creatorID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&creatorID)
	if err != nil {
		return &gen.DeleteOwlChatResponse{Success: false, Message: "chat not found"}, nil
	}
	if creatorID != req.UserId {
		return &gen.DeleteOwlChatResponse{Success: false, Message: "not your chat"}, nil
	}

	_, _ = s.db.Exec("DELETE FROM owl_messages WHERE chat_id = $1", req.ChatId)
	_, _ = s.db.Exec("DELETE FROM owl_chat_settings WHERE chat_id = $1", req.ChatId)
	_, err = s.db.Exec("DELETE FROM chats WHERE id = $1", req.ChatId)
	if err != nil {
		return &gen.DeleteOwlChatResponse{Success: false, Message: "delete failed: " + err.Error()}, nil
	}

	return &gen.DeleteOwlChatResponse{Success: true, Message: "OK"}, nil
}

func (s *server) GetOwlHistory(_ context.Context, req *gen.GetOwlHistoryRequest) (*gen.GetOwlHistoryResponse, error) {
	if req.ChatId == "" {
		return &gen.GetOwlHistoryResponse{}, nil
	}

	// Verify ownership — only by creator_id (UUID)
	var ownerID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&ownerID)
	if err != nil {
		return &gen.GetOwlHistoryResponse{}, nil
	}
	if req.UserId != "" && ownerID != req.UserId {
		return &gen.GetOwlHistoryResponse{}, nil
	}

	rows, err := s.db.Query(
		"SELECT role, content, created_at FROM owl_messages WHERE chat_id = $1 ORDER BY created_at ASC",
		req.ChatId,
	)
	if err != nil {
		return &gen.GetOwlHistoryResponse{}, nil
	}
	defer rows.Close()

	var messages []*gen.OwlHistoryMessage
	for rows.Next() {
		var role, content string
		var createdAt time.Time
		if err := rows.Scan(&role, &content, &createdAt); err == nil {
			messages = append(messages, &gen.OwlHistoryMessage{
				Role:      role,
				Content:   content,
				CreatedAt: createdAt.Format(time.RFC3339),
			})
		}
	}

	return &gen.GetOwlHistoryResponse{Messages: messages}, nil
}

func (s *server) UpdateOwlSettings(_ context.Context, req *gen.UpdateOwlSettingsRequest) (*gen.UpdateOwlSettingsResponse, error) {
	if req.ChatId == "" || req.UserId == "" {
		return &gen.UpdateOwlSettingsResponse{Success: false, Message: "chat_id and user_id required"}, nil
	}

	// Verify ownership — only by creator_id (UUID). creator_username is unreliable (username can change).
	var ownerID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&ownerID)
	if err != nil {
		return &gen.UpdateOwlSettingsResponse{Success: false, Message: "chat not found"}, nil
	}
	if ownerID != req.UserId {
		return &gen.UpdateOwlSettingsResponse{Success: false, Message: "not your chat"}, nil
	}

	owlSessions.saveSettings(req.ChatId, req.ApiKey, req.Model)
	return &gen.UpdateOwlSettingsResponse{Success: true, Message: "OK"}, nil
}

func (s *server) GetOwlSettings(_ context.Context, req *gen.GetOwlSettingsRequest) (*gen.GetOwlSettingsResponse, error) {
	if req.ChatId == "" || req.UserId == "" {
		return &gen.GetOwlSettingsResponse{}, nil
	}

	// Verify ownership — only by creator_id (UUID)
	var ownerID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&ownerID)
	if err != nil {
		return &gen.GetOwlSettingsResponse{}, nil
	}
	if ownerID != req.UserId {
		return &gen.GetOwlSettingsResponse{}, nil
	}

	settings := owlSessions.getSettings(req.ChatId)
	freeModels, _ := s.db.GetFreeModels()
	fmInfos := make([]*gen.FreeModelInfo, 0, len(freeModels))
	for _, m := range freeModels {
		fmInfos = append(fmInfos, &gen.FreeModelInfo{
			ModelId:     m.ModelID,
			DisplayName: m.DisplayName,
			SortOrder:   int32(m.SortOrder),
		})
	}

	// Calculate remaining requests
	remaining := int32(freeTierRateLimiter.remaining(req.UserId))
	limit := int32(20)
	windowSec := int32(3600)
	if settings.UserAPIKey != "" {
		remaining = int32(owlRateLimiter.remaining(req.UserId))
		limit = 10
		windowSec = 60
	}

	return &gen.GetOwlSettingsResponse{
		ApiKey:           settings.UserAPIKey,
		Model:            settings.Model,
		IsUsingCustomKey: settings.UserAPIKey != "",
		FreeModels:       fmInfos,
		Remaining:        remaining,
		Limit:            limit,
		WindowSeconds:    windowSec,
	}, nil
}

func (s *server) getAIChatManager() *AIChatManager {
	if s.aiChatManager == nil {
		s.aiChatManager = NewAIChatManager(s.db.DB)
	}
	return s.aiChatManager
}

func (s *server) ChatWithAI(req *gen.AIChatRequest, stream gen.ChatService_ChatWithAIServer) error {
	userID := req.UserId
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
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
		log.Printf("[ChatWithAI] routing to Hermes orchestrator: session=%s user=%s", sessionID, userID)

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
			log.Printf("[ChatWithAI] orchestrator error: %v", err)
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
		log.Printf("[ChatWithAI] routing to OWL: session=%s user=%s", sessionID, userID)

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
			log.Printf("[ChatWithAI] OpenRouter error: %v", err)
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

func (s *server) GetAIChatHistory(_ context.Context, req *gen.GetAIChatHistoryRequest) (*gen.GetAIChatHistoryResponse, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.GetAIChatHistoryResponse{}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.GetAIChatHistoryResponse{}, nil
	}
	if session.UserID != req.UserId {
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

func (s *server) GetAIChatSettings(_ context.Context, req *gen.GetAIChatSettingsRequest) (*gen.AIChatSettings, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.AIChatSettings{}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.AIChatSettings{}, nil
	}
	if session.UserID != req.UserId {
		return &gen.AIChatSettings{}, nil
	}

	settings, _ := manager.GetSettings(req.SessionId)

	// Calculate remaining requests
	remaining := int32(freeTierRateLimiter.remaining(req.UserId))
	limit := int32(20)
	windowSec := int32(3600)
	if settings.UserAPIKey != "" {
		remaining = int32(owlRateLimiter.remaining(req.UserId))
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

func (s *server) UpdateAIChatSettings(_ context.Context, req *gen.UpdateAIChatSettingsRequest) (*gen.UpdateAIChatSettingsResponse, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "session_id and user_id required"}, nil
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "session not found"}, nil
	}
	if session.UserID != req.UserId {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "not your session"}, nil
	}

	err = manager.SaveSettings(req.SessionId, req.ApiKey, req.Model)
	if err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: err.Error()}, nil
	}

	return &gen.UpdateAIChatSettingsResponse{Success: true, Message: "OK"}, nil
}

func (s *server) GetHermesSettings(_ context.Context, req *gen.GetHermesSettingsRequest) (*gen.GetHermesSettingsResponse, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.GetHermesSettingsResponse{}, nil
	}

	// Verify ownership — only by creator_id (UUID)
	var ownerID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'hermes'", req.SessionId).Scan(&ownerID)
	if err != nil {
		return &gen.GetHermesSettingsResponse{}, nil
	}
	if ownerID != req.UserId {
		return &gen.GetHermesSettingsResponse{}, nil
	}

	settings := hermesSettings.getSettings(req.SessionId)

	// Calculate remaining requests
	remaining := int32(freeTierRateLimiter.remaining(req.UserId))
	limit := int32(20)
	windowSec := int32(3600)
	if settings.UserAPIKey != "" {
		remaining = int32(owlRateLimiter.remaining(req.UserId))
		limit = 10
		windowSec = 60
	}

	return &gen.GetHermesSettingsResponse{
		ApiKey:           settings.UserAPIKey,
		Model:            settings.Model,
		IsUsingCustomKey: settings.UserAPIKey != "",
		Remaining:        remaining,
		Limit:            limit,
		WindowSeconds:    windowSec,
	}, nil
}

func (s *server) UpdateHermesSettings(_ context.Context, req *gen.UpdateHermesSettingsRequest) (*gen.UpdateHermesSettingsResponse, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.UpdateHermesSettingsResponse{Success: false, Message: "session_id and user_id required"}, nil
	}

	// Verify ownership — only by creator_id (UUID)
	var ownerID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'hermes'", req.SessionId).Scan(&ownerID)
	if err != nil {
		return &gen.UpdateHermesSettingsResponse{Success: false, Message: "session not found"}, nil
	}
	if ownerID != req.UserId {
		return &gen.UpdateHermesSettingsResponse{Success: false, Message: "not your session"}, nil
	}

	hermesSettings.saveSettings(req.SessionId, req.ApiKey, req.Model)
	return &gen.UpdateHermesSettingsResponse{Success: true, Message: "OK"}, nil
}

func (s *server) ChatWithOrchestrator(req *gen.OrchestratorRequest, stream gen.ChatService_ChatWithOrchestratorServer) error {
	userID := req.UserId
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}

	chatID := req.SessionId
	if chatID == "" {
		chatID = "hermes-" + userID
	}

	log.Printf("[Lava] chat=%s user=%s session=%s msg=%q", chatID, userID, req.SessionId, truncateString(req.Message, 80))

	// Rate limit check: custom key users get 10/min, free tier gets 20/hour
	hermSet := hermesSettings.getSettings(chatID)
	hasCustomHermesKey := hermSet.UserAPIKey != ""
	if hasCustomHermesKey {
		if !owlRateLimiter.allow(userID) {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded: max 10 requests per minute")
		}
	} else {
		if !freeTierRateLimiter.allow(userID) {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded: max 20 requests per hour for free tier. Set your own API key in chat settings for unlimited access")
		}
	}

	// Check if orchestrator is initialized
	if s.hermesOrchestrator == nil {
		return status.Error(codes.Unavailable, "orchestrator not initialized")
	}

	// Welcome message: only send if user explicitly asks /help
	// (removed auto-welcome on first message to avoid spam)

	// Handle /help command — send welcome message with agent list
	if strings.TrimSpace(req.Message) == "/help" {
		welcomeMsg := s.buildWelcomeMessage()
		log.Printf("[Lava] sending /help welcome message for user=%s", userID)
		if err := stream.Send(&gen.OrchestratorResponse{
			Token:    welcomeMsg,
			Finished: true,
		}); err != nil {
			log.Printf("[Lava] /help send error: %v", err)
			return err
		}
		// Save to DB via AIChatManager
		manager := s.getAIChatManager()
		manager.AddMessage(chatID, "user", req.Message, "")
		manager.AddMessage(chatID, "assistant", welcomeMsg, "")
		return nil
	}

	// Run orchestrator — collect full response for DB saving
	var fullResponse strings.Builder
	log.Printf("[Lava] calling Orchestrate for user=%s chat=%s", userID, chatID)
	err := s.hermesOrchestrator.Orchestrate(stream.Context(), userID, chatID, req.Message,
		func(token string, finished bool) error {
			if !finished && token != "" {
				fullResponse.WriteString(token)
			}
			return stream.Send(&gen.OrchestratorResponse{
				Token:    token,
				Finished: finished,
			})
		})

	if err != nil {
		log.Printf("[Lava] orchestrator error for user %s: %v", userID, err)
		// Refund rate limit slot on failure
		if hasCustomHermesKey {
			owlRateLimiter.cancel(userID)
		} else {
			freeTierRateLimiter.cancel(userID)
		}
		if err := stream.Send(&gen.OrchestratorResponse{
			Token:    "",
			Finished: true,
			Error:    err.Error(),
		}); err != nil { log.Printf("Failed to send orchestrator response: %v", err) }
		return nil
	}

	// Save user message to DB via AIChatManager
	manager := s.getAIChatManager()
	manager.AddMessage(chatID, "user", req.Message, "")

	// Save assistant response to DB (strip agent prefix like "[Support] ")
	assistantResponse := fullResponse.String()
	if idx := strings.Index(assistantResponse, "] "); idx >= 0 && idx < 30 {
		assistantResponse = assistantResponse[idx+2:]
	}
	if assistantResponse != "" {
		manager.AddMessage(chatID, "assistant", assistantResponse, "")
	}

	// Update chat last message
	_, _ = s.db.Exec("UPDATE chats SET last_message_text=$1, last_message_time=NOW() WHERE id=$2",
		truncateString(assistantResponse, 100), chatID)

	log.Printf("[Lava] Orchestrate completed for user=%s", userID)
	return nil
}

func (s *server) ChatWithPipeline(req *gen.PipelineRequest, stream gen.ChatService_ChatWithPipelineServer) error {
	userID := req.UserId
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Rate limit check
	if !owlRateLimiter.allow(userID) {
		return status.Error(codes.ResourceExhausted, "rate limit exceeded: max 10 requests per minute")
	}

	if s.hermesOrchestrator == nil {
		return status.Error(codes.Unavailable, "orchestrator not initialized")
	}

	log.Printf("[Pipeline] user=%s msg=%q images=%d model_hint=%q",
		userID, truncateString(req.Message, 80), len(req.Images), req.ModelHint)

	// Запускаем pipeline
	err := s.hermesOrchestrator.ProcessWithPipeline(
		stream.Context(),
		userID,
		req.Message,
		req.Images,
		func(token string, finished bool) error {
			return stream.Send(&gen.PipelineResponse{
				Token:    token,
				Finished: finished,
			})
		},
	)

	if err != nil {
		log.Printf("[Pipeline] error for user %s: %v", userID, err)
		// Refund rate limit slot on failure
		owlRateLimiter.cancel(userID)
		if err := stream.Send(&gen.PipelineResponse{
			Finished: true,
			Error:    err.Error(),
		}); err != nil {
			log.Printf("Failed to send pipeline response: %v", err)
		}
		return nil
	}

	return nil
}

func (s *server) buildWelcomeMessage() string {
	if s.hermesOrchestrator == nil || s.hermesOrchestrator.registry == nil {
		return "Добро пожаловать в Лава ИИ! Оркестратор временно недоступен."
	}

	var sb strings.Builder
	sb.WriteString("👋 Добро пожаловать в **Лава ИИ** — мульти-агентный AI оркестратор!\n\n")
	sb.WriteString("Я автоматически маршрутизирую ваши запросы к специализированным агентам.\n\n")
	sb.WriteString("**Доступные агенты:**\n")

	for _, agent := range s.hermesOrchestrator.registry.GetAll() {
		icon := agent.Icon
		if icon == "" {
			icon = "🤖"
		}
		sb.WriteString(fmt.Sprintf("%s **%s** — %s\n", icon, agent.Name, agent.Description))
	}

	sb.WriteString("\nПросто напишите ваш вопрос, и я выберу подходящего агента!\n")
	sb.WriteString("Или укажите агента напрямую: `@developer напиши код...`")

	return sb.String()
}

func (s *server) GetOrchestratorHistory(_ context.Context, req *gen.GetOrchestratorHistoryRequest) (*gen.GetOrchestratorHistoryResponse, error) {
	userID := req.UserId
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	manager := s.getAIChatManager()

	// Verify ownership
	session, err := manager.GetSession(req.SessionId)
	if err != nil {
		return &gen.GetOrchestratorHistoryResponse{}, nil
	}
	if session.UserID != userID {
		return &gen.GetOrchestratorHistoryResponse{}, nil
	}

	// Load from DB via AIChatManager
	dbMessages, err := manager.GetHistory(req.SessionId, 50)
	if err != nil {
		return &gen.GetOrchestratorHistoryResponse{}, nil
	}

	messages := make([]*gen.HermesChatMessage, 0, len(dbMessages))
	for _, msg := range dbMessages {
		messages = append(messages, &gen.HermesChatMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format(time.RFC3339),
		})
	}

	return &gen.GetOrchestratorHistoryResponse{Messages: messages}, nil
}

func (s *server) ListAgents(_ context.Context, req *gen.ListAgentsRequest) (*gen.ListAgentsResponse, error) {
	userID := req.UserId
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if s.hermesOrchestrator == nil {
		return &gen.ListAgentsResponse{}, nil
	}

	// Load custom agents from DB
	rows, err := s.db.Query(
		"SELECT id, name, COALESCE(system_prompt, ''), COALESCE(model, ''), COALESCE(max_tokens, 2048) FROM hermes_custom_agents WHERE user_id = $1 ORDER BY created_at ASC",
		userID)
	if err != nil {
		log.Printf("[Lava] ListAgents DB error: %v", err)
		return &gen.ListAgentsResponse{}, nil
	}
	defer rows.Close()

	var agents []*gen.AgentInfo
	for rows.Next() {
		var a gen.AgentInfo
		var model string
		var maxTokens int32
		if err := rows.Scan(&a.Id, &a.Name, &a.SystemPrompt, &model, &maxTokens); err == nil {
			a.IsPreset = false
			a.Model = model
			a.MaxTokens = maxTokens
			agents = append(agents, &a)
		}
	}

	return &gen.ListAgentsResponse{Agents: agents}, nil
}

func (s *server) ListAgentPresets(_ context.Context, _ *gen.ListAgentPresetsRequest) (*gen.ListAgentPresetsResponse, error) {
	if s.hermesOrchestrator == nil || s.hermesOrchestrator.registry == nil {
		return &gen.ListAgentPresetsResponse{}, nil
	}

	presets := s.hermesOrchestrator.registry.GetPresets()
	result := make([]*gen.AgentPresetInfo, 0, len(presets))
	for _, p := range presets {
		result = append(result, &gen.AgentPresetInfo{
			Id:          p.ID,
			Name:        p.Name,
			Role:        p.ID,
			Description: p.Description,
			Icon:        p.Icon,
			MaxTokens:   int32(p.MaxTokens),
		})
	}

	return &gen.ListAgentPresetsResponse{Presets: result}, nil
}

func (s *server) CreateAgent(_ context.Context, req *gen.CreateAgentRequest) (*gen.CreateAgentResponse, error) {
	userID := req.UserId
	if userID == "" {
		return &gen.CreateAgentResponse{Success: false, Error: "user_id is required"}, nil
	}

	if s.hermesOrchestrator == nil {
		return &gen.CreateAgentResponse{Success: false, Error: "orchestrator not initialized"}, nil
	}

	agentID := "custom-" + userID + "-" + req.PresetId + "-" + fmt.Sprintf("%d", time.Now().Unix())

	_, err := s.db.Exec(
		"INSERT INTO hermes_custom_agents (id, user_id, created_by, preset_id, name, system_prompt, model, max_tokens) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		agentID, userID, userID, req.PresetId, req.Name, req.SystemPrompt, req.Model, req.MaxTokens,
	)
	if err != nil {
		log.Printf("[Lava] CreateAgent DB error: %v", err)
		return &gen.CreateAgentResponse{Success: false, Error: err.Error()}, nil
	}

	// Reload custom agents in registry
	s.hermesOrchestrator.registry.LoadCustomAgents(s.db.DB)

	log.Printf("[Lava] created agent %s for user %s", agentID, userID)
	return &gen.CreateAgentResponse{Success: true, AgentId: agentID}, nil
}

func (s *server) UpdateAgent(_ context.Context, req *gen.UpdateAgentRequest) (*gen.UpdateAgentResponse, error) {
	userID := req.UserId
	if userID == "" || req.AgentId == "" {
		return &gen.UpdateAgentResponse{Success: false, Error: "agent_id and user_id required"}, nil
	}

	// Verify ownership
	var owner string
	err := s.db.QueryRow("SELECT user_id FROM hermes_custom_agents WHERE id = $1", req.AgentId).Scan(&owner)
	if err != nil || owner != userID {
		return &gen.UpdateAgentResponse{Success: false, Error: "not your agent"}, nil
	}

	_, err = s.db.Exec(
		"UPDATE hermes_custom_agents SET name=$1, system_prompt=$2, model=$3, max_tokens=$4 WHERE id=$5",
		req.Name, req.SystemPrompt, req.Model, req.MaxTokens, req.AgentId,
	)
	if err != nil {
		return &gen.UpdateAgentResponse{Success: false, Error: err.Error()}, nil
	}

	// Reload
	if s.hermesOrchestrator != nil {
		s.hermesOrchestrator.registry.LoadCustomAgents(s.db.DB)
	}

	return &gen.UpdateAgentResponse{Success: true}, nil
}

func (s *server) DeleteAgent(_ context.Context, req *gen.DeleteAgentRequest) (*gen.DeleteAgentResponse, error) {
	userID := req.UserId
	if userID == "" || req.AgentId == "" {
		return &gen.DeleteAgentResponse{Success: false, Error: "agent_id and user_id required"}, nil
	}

	// Verify ownership
	var owner string
	err := s.db.QueryRow("SELECT user_id FROM hermes_custom_agents WHERE id = $1", req.AgentId).Scan(&owner)
	if err != nil || owner != userID {
		return &gen.DeleteAgentResponse{Success: false, Error: "not your agent"}, nil
	}

	_, err = s.db.Exec("DELETE FROM hermes_custom_agents WHERE id = $1", req.AgentId)
	if err != nil {
		return &gen.DeleteAgentResponse{Success: false, Error: err.Error()}, nil
	}

	// Reload
	if s.hermesOrchestrator != nil {
		s.hermesOrchestrator.registry.LoadCustomAgents(s.db.DB)
	}

	return &gen.DeleteAgentResponse{Success: true}, nil
}

func (s *server) ListUserAgents(_ context.Context, req *gen.ListUserAgentsRequest) (*gen.ListUserAgentsResponse, error) {
	userID := req.UserId
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if s.hermesOrchestrator == nil {
		return &gen.ListUserAgentsResponse{}, nil
	}

	// Start with presets
	presets := s.hermesOrchestrator.registry.GetPresets()
	result := make([]*gen.AgentInfo, 0, len(presets))
	for _, p := range presets {
		result = append(result, &gen.AgentInfo{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			IsPreset:    true,
			Model:       p.Model,
			MaxTokens:   int32(p.MaxTokens),
		})
	}

	// Add custom agents from DB
	rows, err := s.db.Query(
		"SELECT id, name, COALESCE(system_prompt, ''), COALESCE(model, ''), COALESCE(max_tokens, 2048) FROM hermes_custom_agents WHERE user_id = $1 ORDER BY created_at ASC",
		userID)
	if err == nil {
		for rows.Next() {
			var a gen.AgentInfo
			var model string
			var maxTokens int32
			if err := rows.Scan(&a.Id, &a.Name, &a.SystemPrompt, &model, &maxTokens); err == nil {
				a.IsPreset = false
				a.Model = model
				a.MaxTokens = maxTokens
				result = append(result, &a)
			}
		}
		rows.Close()
	}

	return &gen.ListUserAgentsResponse{Agents: result}, nil
}

func (s *server) CreateHermesSession(_ context.Context, req *gen.CreateHermesSessionRequest) (*gen.CreateHermesSessionResponse, error) {
	userID := req.UserId
	log.Printf("[Lava] CreateHermesSession: user_id=%q name=%q", userID, req.Name)
	if userID == "" {
		log.Printf("[Lava] CreateHermesSession: ERROR empty user_id")
		return &gen.CreateHermesSessionResponse{Success: false, Error: "user_id is required"}, nil
	}

	// Resolve username → UUID if needed (hermes_sessions.user_id is UUID type)
	if _, err := uuid.Parse(userID); err != nil {
		if uid, err := s.db.GetUserIdByUsername(userID); err == nil && uid != "" {
			log.Printf("[Lava] CreateHermesSession: resolved username %q → UUID %q", userID, uid)
			userID = uid
		} else {
			log.Printf("[Lava] CreateHermesSession: WARNING could not resolve username %q to UUID, using as-is", userID)
		}
	}

	// Generate sequential number for this user's Hermes sessions
	num := s.getNextChatNumber(userID, "hermes")
	name := req.Name
	if name == "" {
		name = generateChatName("hermes", num)
	}

	sessionID := "hermes-" + uuid.New().String()

	_, err := s.db.Exec(
		"INSERT INTO hermes_sessions (id, user_id, name) VALUES ($1, $2, $3)",
		sessionID, userID, name,
	)
	if err != nil {
		log.Printf("[Lava] CreateHermesSession: DB error (hermes_sessions): %v", err)
		return &gen.CreateHermesSessionResponse{Success: false, Error: err.Error()}, nil
	}

	// Also insert into chats so the session appears in chat list and can be deleted
	username := userID
	if uname, err := s.db.GetUsernameByID(userID); err == nil && uname != "" {
		username = uname
	}
	participantsJSON, _ := json.Marshal([]string{userID})
	_, err = s.db.Exec(
		"INSERT INTO chats (id, name, type, participants, creator_username, creator_id) VALUES ($1, $2, 'hermes', $3, $4, $5) ON CONFLICT (id) DO NOTHING",
		sessionID, name, string(participantsJSON), username, userID,
	)
	if err != nil {
		log.Printf("[Lava] CreateHermesSession: WARNING failed to insert into chats: %v", err)
	}

	log.Printf("[Lava] CreateHermesSession: OK session_id=%s name=%s", sessionID, name)
	return &gen.CreateHermesSessionResponse{Success: true, SessionId: sessionID, Name: name}, nil
}

func (s *server) DeleteHermesSession(_ context.Context, req *gen.DeleteHermesSessionRequest) (*gen.DeleteHermesSessionResponse, error) {
	if req.SessionId == "" || req.UserId == "" {
		return &gen.DeleteHermesSessionResponse{Success: false, Error: "session_id and user_id required"}, nil
	}

	_, err := s.db.Exec("DELETE FROM hermes_sessions WHERE id = $1 AND user_id = $2", req.SessionId, req.UserId)
	if err != nil {
		return &gen.DeleteHermesSessionResponse{Success: false, Error: err.Error()}, nil
	}

	return &gen.DeleteHermesSessionResponse{Success: true}, nil
}

func (s *server) ListRemoteAgents(_ context.Context, _ *gen.ListRemoteAgentsRequest) (*gen.ListRemoteAgentsResponse, error) {
	if s.hermesOrchestrator == nil || s.hermesOrchestrator.remoteManager == nil {
		return &gen.ListRemoteAgentsResponse{}, nil
	}

	agents := s.hermesOrchestrator.remoteManager.GetAllAgents()
	result := make([]*gen.RemoteAgentInfo, 0, len(agents))
	for _, a := range agents {
		result = append(result, &gen.RemoteAgentInfo{
			Id:            a.ID,
			Name:          a.Name,
			Host:          a.Host,
			IpAddress:     a.IPAddress,
			Os:            a.OS,
			Status:        a.Status,
			Capabilities:  a.Capabilities,
			ActiveTasks:   int32(a.ActiveTasks),
			LastHeartbeat: a.LastHeartbeat.Format(time.RFC3339),
		})
	}

	return &gen.ListRemoteAgentsResponse{Agents: result}, nil
}

func (s *server) DeployAgentTask(_ context.Context, req *gen.DeployAgentTaskRequest) (*gen.DeployAgentTaskResponse, error) {
	if s.hermesOrchestrator == nil || s.hermesOrchestrator.remoteManager == nil {
		return &gen.DeployAgentTaskResponse{Success: false, Error: "remote manager not available"}, nil
	}

	taskID := uuid.New().String()[:12]
	task := &RemoteTask{
		ID:           taskID,
		AgentID:      req.AgentId,
		Type:         req.TaskType,
		Params:       req.Params,
		WorkingDir:   req.WorkingDir,
		TimeoutSec:   int(req.TimeoutSec),
		StreamOutput: true,
	}

	if err := s.hermesOrchestrator.remoteManager.SendTask(task); err != nil {
		return &gen.DeployAgentTaskResponse{Success: false, Error: err.Error()}, nil
	}

	// Ждём результат (blocking, с таймаутом)
	timeout := time.Duration(task.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	result := s.hermesOrchestrator.remoteManager.WaitForResult(taskID, timeout)
	if result == nil {
		return &gen.DeployAgentTaskResponse{
			Success: true, TaskId: taskID,
			Error: "task sent but no result yet (timeout)",
		}, nil
	}

	return &gen.DeployAgentTaskResponse{
		Success:    result.Status == "success",
		TaskId:     taskID,
		Error:      result.Error,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExitCode:   int32(result.ExitCode),
		DurationMs: int64(result.Duration.Milliseconds()),
	}, nil
}

func (s *server) GetRemoteAgentStatus(_ context.Context, req *gen.GetRemoteAgentStatusRequest) (*gen.GetRemoteAgentStatusResponse, error) {
	if s.hermesOrchestrator == nil || s.hermesOrchestrator.remoteManager == nil {
		return &gen.GetRemoteAgentStatusResponse{Status: "unavailable"}, nil
	}

	agent := s.hermesOrchestrator.remoteManager.GetAgent(req.AgentId)
	if agent == nil {
		return &gen.GetRemoteAgentStatusResponse{Status: "not_found"}, nil
	}

	return &gen.GetRemoteAgentStatusResponse{
		Status:        agent.Status,
		ActiveTasks:   int32(agent.ActiveTasks),
		LastHeartbeat: agent.LastHeartbeat.Format(time.RFC3339),
	}, nil
}

func (s *server) GetAIChats(_ context.Context, req *gen.GetAIChatsRequest) (*gen.GetAIChatsResponse, error) {
	if req.UserId == "" {
		return &gen.GetAIChatsResponse{}, nil
	}

	var chats []*gen.AIChatInfo

	// OWL chats from chats table
	owlRows, err := s.db.Query(
		"SELECT id, name, type, created_at FROM chats WHERE type = 'owl' AND creator_id = $1 ORDER BY created_at ASC",
		req.UserId,
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
		req.UserId,
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

func (s *server) RenameAIChat(_ context.Context, req *gen.RenameAIChatRequest) (*gen.RenameAIChatResponse, error) {
	if req.ChatId == "" || req.UserId == "" || req.NewName == "" {
		return &gen.RenameAIChatResponse{Success: false, Error: "chat_id, user_id and new_name required"}, nil
	}

	// Try OWL chat first
	var creatorID string
	err := s.db.QueryRow("SELECT creator_id FROM chats WHERE id = $1 AND type = 'owl'", req.ChatId).Scan(&creatorID)
	if err == nil && creatorID == req.UserId {
		_, err = s.db.Exec("UPDATE chats SET name = $1 WHERE id = $2", req.NewName, req.ChatId)
		if err != nil {
			return &gen.RenameAIChatResponse{Success: false, Error: err.Error()}, nil
		}
		return &gen.RenameAIChatResponse{Success: true}, nil
	}

	// Try Hermes session
	var userID string
	err = s.db.QueryRow("SELECT user_id FROM hermes_sessions WHERE id = $1", req.ChatId).Scan(&userID)
	if err == nil && userID == req.UserId {
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
		log.Printf("[FreeModels] GetFreeModels error: %v", err)
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

func (s *server) SetFreeModel(_ context.Context, req *gen.SetFreeModelRequest) (*gen.SetFreeModelResponse, error) {
	if req.AdminUserId == "" {
		return &gen.SetFreeModelResponse{Success: false, Error: "admin_user_id required"}, nil
	}
	if !s.db.IsSuperAdmin(req.AdminUserId) {
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

func (s *server) RemoveFreeModel(_ context.Context, req *gen.RemoveFreeModelRequest) (*gen.RemoveFreeModelResponse, error) {
	if req.AdminUserId == "" {
		return &gen.RemoveFreeModelResponse{Success: false, Error: "admin_user_id required"}, nil
	}
	if !s.db.IsSuperAdmin(req.AdminUserId) {
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
