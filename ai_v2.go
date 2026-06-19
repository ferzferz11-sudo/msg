package main

// ai_v2.go — AI Gateway v2: session management, streaming, routing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AIGateway handles all AI chat operations
type AIGateway struct {
	db           *sql.DB
	executor     *AgentExecutor
	tools        *ToolRegistry
	router       *HybridRouter
	mu           sync.RWMutex
	rateLimiters map[string]*rateLimiter
}

// NewAIGateway creates and initializes the gateway
func NewAIGateway(db *sql.DB) *AIGateway {
	registry := NewProviderRegistry()
	tools := NewToolRegistry(db)
	executor := NewAgentExecutor(db, registry, tools)
	router := NewHybridRouter(db)

	g := &AIGateway{
		db:           db,
		executor:     executor,
		tools:        tools,
		router:       router,
		rateLimiters: make(map[string]*rateLimiter),
	}
	return g
}

// Chat is the main entry point for AI chat
func (g *AIGateway) Chat(ctx context.Context, req *ChatRequest, streamFn func(token string, finished bool) error) error {
	userID := req.UserID

	// 1. Load or create chat
	chat, err := g.loadOrCreateChat(ctx, req)
	if err != nil {
		return err
	}

	// 2. Verify ownership
	if chat.UserID != userID {
		return fmt.Errorf("permission denied")
	}

	// 3. Load agent
	agent, err := g.resolveAgent(ctx, chat, req.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %v", err)
	}

	// 4. Rate limit
	if err := g.checkRateLimit(agent, userID); err != nil {
		return err
	}

	// 5. Save user message
	if err := g.saveUserMessage(ctx, chat.ID, req.Message); err != nil {
		return fmt.Errorf("failed to save message: %v", err)
	}

	// 6. Load history
	history, err := g.loadHistory(ctx, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to load history: %v", err)
	}

	// 7. Build messages for provider
	messages := g.buildMessages(chat, history, req.Message)

	// 8. Get settings
	settings := &AIChatSettings{}

	// 9. Execute
	var fullResponse string
	err = g.executor.Execute(ctx, agent, messages, settings, func(token string, finished bool) error {
		if !finished {
			fullResponse += token
		}
		return streamFn(token, finished)
	})

	// 10. Refund on error
	if err != nil {
		g.refundRateLimit(agent, userID)
		return err
	}

	// 11. Save assistant response
	g.saveAssistantMessage(ctx, chat.ID, agent.ID, fullResponse)

	// 12. Update chats table for UI
	g.updateChatsLastMessage(ctx, chat.ID, fullResponse)

	return nil
}

// ChatRequest — input for Chat
type ChatRequest struct {
	UserID   string
	ChatID   string // empty = create new
	Message  string
	AgentID  string
	ChatType string // simple, agent, pipeline
}

func (g *AIGateway) loadOrCreateChat(ctx context.Context, req *ChatRequest) (*AIChatV2, error) {
	if req.ChatID != "" {
		return g.dbGetChat(req.ChatID)
	}

	chatType := req.ChatType
	if chatType == "" {
		chatType = "simple"
		if req.AgentID != "" {
			chatType = "agent"
		}
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "assistant"
	}

	chatID := fmt.Sprintf("ai-chat-%s", uuid.New().String()[:8])
	name := g.generateChatName(chatType)

	chat := &AIChatV2{
		ID:       chatID,
		UserID:   req.UserID,
		ChatType: chatType,
		Name:     name,
		AgentID:  agentID,
	}

	if err := g.dbCreateChat(chat); err != nil {
		return nil, err
	}

	g.db.Exec(`INSERT INTO chats (id, name, type, participants, creator_id) VALUES ($1, $2, 'ai', '[]', $3::uuid) ON CONFLICT DO NOTHING`,
		chatID, name, req.UserID)

	return chat, nil
}

func (g *AIGateway) resolveAgent(ctx context.Context, chat *AIChatV2, forcedAgentID string) (*AgentV2, error) {
	agentID := forcedAgentID
	if agentID == "" {
		agentID = chat.AgentID
	}
	if agentID == "" {
		agentID = "assistant"
	}
	return g.dbGetAgent(agentID)
}

func (g *AIGateway) checkRateLimit(agent *AgentV2, userID string) error {
	limiter := g.getRateLimiter(agent)
	if limiter == nil {
		return nil
	}
	if !limiter.allow(userID) {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

func (g *AIGateway) refundRateLimit(agent *AgentV2, userID string) {
	limiter := g.getRateLimiter(agent)
	if limiter != nil {
		limiter.cancel(userID)
	}
}

func (g *AIGateway) getRateLimiter(agent *AgentV2) *rateLimiter {
	key := agent.ID
	g.mu.RLock()
	limiter, ok := g.rateLimiters[key]
	g.mu.RUnlock()
	if ok {
		return limiter
	}

	limit := 10
	if agent.RateLimit != nil {
		limit = *agent.RateLimit
	}
	limiter = newRateLimiter(limit, time.Minute)

	g.mu.Lock()
	g.rateLimiters[key] = limiter
	g.mu.Unlock()
	return limiter
}

func (g *AIGateway) loadHistory(ctx context.Context, chatID string) ([]AIMessageInput, error) {
	msgs, err := g.dbGetMessages(chatID, 50)
	if err != nil {
		return nil, err
	}
	var history []AIMessageInput
	for _, m := range msgs {
		history = append(history, AIMessageInput{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return history, nil
}

func (g *AIGateway) buildMessages(chat *AIChatV2, history []AIMessageInput, userMessage string) []AIMessageInput {
	var messages []AIMessageInput
	sysPrompt := chat.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a helpful AI assistant."
	}
	messages = append(messages, AIMessageInput{Role: "system", Content: sysPrompt})
	messages = append(messages, history...)
	messages = append(messages, AIMessageInput{Role: "user", Content: userMessage})
	return messages
}

func (g *AIGateway) saveUserMessage(ctx context.Context, chatID, message string) error {
	return g.dbAddMessage(&AIMessageV2{
		ChatID:  chatID,
		Role:    "user",
		Content: message,
	})
}

func (g *AIGateway) saveAssistantMessage(ctx context.Context, chatID, agentID, content string) {
	g.dbAddMessage(&AIMessageV2{
		ChatID:  chatID,
		Role:    "assistant",
		AgentID: agentID,
		Content: content,
	})
}

func (g *AIGateway) updateChatsLastMessage(ctx context.Context, chatID, content string) {
	truncated := content
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	g.db.Exec(`UPDATE chats SET last_message_text = $1, last_message_time = NOW() WHERE id = $2`, truncated, chatID)
}

func (g *AIGateway) generateChatName(chatType string) string {
	switch chatType {
	case "simple":
		return "AI Chat"
	case "agent":
		return "Agent Chat"
	case "pipeline":
		return "Pipeline Chat"
	}
	return "AI Chat"
}

// ======= DB wrappers =======

func (g *AIGateway) dbGetChat(id string) (*AIChatV2, error) {
	var c AIChatV2
	var sJSON string
	err := g.db.QueryRow(`SELECT id, user_id, chat_type, name, agent_id, model, system_prompt, bound_agent_id, bind_until_msg, settings, created_at, updated_at FROM ai_chats_v2 WHERE id = $1`, id).
		Scan(&c.ID, &c.UserID, &c.ChatType, &c.Name, &c.AgentID, &c.Model, &c.SystemPrompt, &c.BoundAgentID, &c.BindUntilMsg, &sJSON, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Settings = make(map[string]any)
	if sJSON != "" {
		json.Unmarshal([]byte(sJSON), &c.Settings)
	}
	return &c, nil
}

func (g *AIGateway) dbCreateChat(c *AIChatV2) error {
	sJSON := "{}"
	_, err := g.db.Exec(`INSERT INTO ai_chats_v2 (id, user_id, chat_type, name, agent_id, model, system_prompt, settings) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, c.UserID, c.ChatType, c.Name, c.AgentID, c.Model, c.SystemPrompt, sJSON)
	return err
}

func (g *AIGateway) dbGetAgent(id string) (*AgentV2, error) {
	return GetAgentV2FromDB(g.db, id)
}

func (g *AIGateway) dbGetMessages(chatID string, limit int) ([]*AIMessageV2, error) {
	rows, err := g.db.Query(`SELECT id, chat_id, role, content, agent_id, token_count, model_used, created_at FROM ai_messages_v2 WHERE chat_id = $1 ORDER BY created_at ASC LIMIT $2`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*AIMessageV2
	for rows.Next() {
		var m AIMessageV2
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.AgentID, &m.TokenCount, &m.ModelUsed, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (g *AIGateway) dbAddMessage(m *AIMessageV2) error {
	_, err := g.db.Exec(`INSERT INTO ai_messages_v2 (chat_id, role, content, agent_id, token_count, model_used) VALUES ($1,$2,$3,$4,$5,$6)`,
		m.ChatID, m.Role, m.Content, m.AgentID, m.TokenCount, m.ModelUsed)
	return err
}
