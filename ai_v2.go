package main

// ai_v2.go — AI Gateway v2: session management, streaming, routing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"LavenderMessenger/core/rag"
	"LavenderMessenger/core/rag/memory"
	"LavenderMessenger/core/rag/qdrant"
)

// AIGateway handles all AI chat operations
type AIGateway struct {
	db           *sql.DB
	executor     *AgentExecutor
	tools        *ToolRegistry
	router       *HybridRouter
	rag          rag.RAGPipeline
	embedder     rag.EmbeddingService
	vectorDB     rag.VectorSearch
	mu           sync.RWMutex
	rateLimiters map[string]*RedisRateLimiter
	userMu       sync.Map // per-user mutex for session dedup
}

// NewAIGateway creates and initializes the gateway
func NewAIGateway(db *sql.DB) *AIGateway {
	registry := NewProviderRegistry()
	tools := NewToolRegistry(db)
	executor := NewAgentExecutor(db, registry, tools)
	router := NewHybridRouter(db)

	ragPipeline, embedder, vectorDB := initRAG()

	g := &AIGateway{
		db:           db,
		executor:     executor,
		tools:        tools,
		router:       router,
		rag:          ragPipeline,
		embedder:     embedder,
		vectorDB:     vectorDB,
		rateLimiters: make(map[string]*RedisRateLimiter),
	}
	return g
}

func initRAG() (rag.RAGPipeline, rag.EmbeddingService, rag.VectorSearch) {
	dim := 1536 // text-embedding-3-small dimensions

	// Try Qdrant first
	qdrantClient := qdrant.NewClient("rag", dim)
	if qdrantClient != nil && qdrantClient.IsAvailable() {
		embedder := qdrant.NewOpenAIEmbeddingService(dim)
		if embedder != nil {
			log.Printf("[RAG] production mode: Qdrant + OpenAI embeddings")
			return memory.NewInMemoryRAGPipeline(embedder, qdrantClient), embedder, qdrantClient
		}
		log.Printf("[RAG] Qdrant available but no OPENAI_API_KEY, using in-memory embeddings")
		inMemEmb := memory.NewInMemoryEmbeddingService(dim)
		return memory.NewInMemoryRAGPipeline(inMemEmb, qdrantClient), inMemEmb, qdrantClient
	}

	// Fallback: in-memory everything
	log.Printf("[RAG] in-memory mode (set QDRANT_URL + OPENAI_API_KEY for production)")
	inMemEmb := memory.NewInMemoryEmbeddingService(dim)
	inMemVDB := memory.NewInMemoryVectorDB(dim)
	return memory.NewInMemoryRAGPipeline(inMemEmb, inMemVDB), inMemEmb, inMemVDB
}

// Chat is the main entry point for AI chat
func (g *AIGateway) Chat(ctx context.Context, req *ChatRequest, streamFn StreamFn) error {
	userID := req.UserID

	// Per-user lock to prevent concurrent session creation
	userLock := g.getUserLock(userID)
	userLock.Lock()
	defer userLock.Unlock()

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
	messages := g.buildMessages(ctx, chat, agent, history, req.Message)

	// 8. Get settings from chat
	settings := &AIChatSettings{}
	if chat.Settings != nil {
		if apiKey, ok := chat.Settings["user_api_key"].(string); ok {
			settings.UserAPIKey = apiKey
		}
		if model, ok := chat.Settings["model_override"].(string); ok {
			settings.ModelOverride = model
		}
	}

	// 9. Execute
	var fullResponse string
	execResult, err := g.executor.Execute(ctx, agent, messages, settings, func(token string, finished bool) error {
		if !finished {
			fullResponse += token
		}
		return streamFn(token, finished, "", agent.ID, agent.Name)
	})

	// 10. Refund on error
	if err != nil {
		g.refundRateLimit(agent, userID)
		return err
	}

	// 10.5 Send image URL if present (e.g. Reve image generation)
	if execResult != nil && execResult.ImageURL != "" {
		if err := streamFn("", true, execResult.ImageURL, agent.ID, agent.Name); err != nil {
			logger.Warnf("[AI] streamFn image: %v", err)
		}
	}

	// 11. Save assistant response with real token count
	tokenCount := 0
	modelUsed := agent.Model
	if execResult != nil {
		tokenCount = execResult.TokenCount
		modelUsed = execResult.ModelUsed
	}
	g.saveAssistantMessage(ctx, chat.ID, agent.ID, fullResponse, tokenCount, modelUsed)

	// 12. Update usage stats
	g.recordUsage(userID, agent.ID, tokenCount)

	// 13. Update chats table for UI
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

// StreamFn is the callback for streaming tokens. token="" + finished=true signals end.
// imageURL is set when the agent produces an image (e.g. Reve).
// agentID and agentName identify which agent produced this token.
type StreamFn func(token string, finished bool, imageURL string, agentID string, agentName string) error

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

func (g *AIGateway) getRateLimiter(agent *AgentV2) *RedisRateLimiter {
	key := agent.ID
	g.mu.RLock()
	limiter, ok := g.rateLimiters[key]
	g.mu.RUnlock()
	if ok {
		return limiter
	}

	g.mu.Lock()
	limiter, ok = g.rateLimiters[key]
	if ok {
		g.mu.Unlock()
		return limiter
	}

	limit := 10
	if agent.RateLimit != nil {
		limit = *agent.RateLimit
	}
	limiter = NewRedisRateLimiter(limit, time.Minute, "rl:ai:"+agent.ID+":")
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

func (g *AIGateway) buildMessages(ctx context.Context, chat *AIChatV2, agent *AgentV2, history []AIMessageInput, userMessage string) []AIMessageInput {
	var messages []AIMessageInput
	sysPrompt := chat.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a helpful AI assistant."
	}
	messages = append(messages, AIMessageInput{Role: "system", Content: sysPrompt})
	messages = append(messages, history...)

	// RAG augmentation: if agent has RAG enabled, search for relevant context
	augmentedMsg := userMessage
	if agent != nil && agent.RAGEnabled && g.rag != nil {
		ragCtx, err := g.rag.BuildContext(ctx, userMessage, nil)
		if err == nil && ragCtx.HasResults {
			augmentedMsg = ragCtx.AugmentedPrompt
			log.Printf("[RAG] augmented query with %d chunks for agent=%s", len(ragCtx.RetrievedChunks), agent.ID)
		}
	}

	messages = append(messages, AIMessageInput{Role: "user", Content: augmentedMsg})
	return messages
}

func (g *AIGateway) saveUserMessage(ctx context.Context, chatID, message string) error {
	err := g.dbAddMessage(&AIMessageV2{
		ChatID:  chatID,
		Role:    "user",
		Content: message,
	})
	if err == nil {
		go g.indexMessageWithTimeout(chatID, "user", "", message)
	}
	return err
}

func (g *AIGateway) saveAssistantMessage(ctx context.Context, chatID, agentID, content string, tokenCount int, modelUsed string) {
	g.dbAddMessage(&AIMessageV2{
		ChatID:     chatID,
		Role:       "assistant",
		AgentID:    agentID,
		Content:    content,
		TokenCount: tokenCount,
		ModelUsed:  modelUsed,
	})
	go g.indexMessageWithTimeout(chatID, "assistant", agentID, content)
}

func (g *AIGateway) indexMessageWithTimeout(chatID, role, agentID, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g.indexMessage(ctx, chatID, role, agentID, content)
}

func (g *AIGateway) indexMessage(ctx context.Context, chatID, role, agentID, content string) {
	if g.embedder == nil || g.vectorDB == nil || content == "" {
		return
	}

	emb, err := g.embedder.EmbedText(ctx, content)
	if err != nil {
		log.Printf("[RAG] indexMessage: embed error: %v", err)
		return
	}

	pointID := uuid.New().String()
	truncated := content
	if len(truncated) > 2000 {
		truncated = truncated[:2000]
	}

	metadata := map[string]any{
		"content":  truncated,
		"chat_id":  chatID,
		"role":     role,
		"agent_id": agentID,
	}

	if err := g.vectorDB.Upsert(ctx, pointID, emb, metadata); err != nil {
		log.Printf("[RAG] indexMessage: upsert error: %v", err)
	}
}

func (g *AIGateway) updateChatsLastMessage(ctx context.Context, chatID, content string) {
	truncated := content
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	g.db.Exec(`UPDATE chats SET last_message_text = $1, last_message_time = NOW() WHERE id = $2`, truncated, chatID)
}

func (g *AIGateway) getUserLock(userID string) *sync.Mutex {
	val, _ := g.userMu.LoadOrStore(userID, &sync.Mutex{})
	return val.(*sync.Mutex)
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

// recordUsage tracks token usage per user per agent
func (g *AIGateway) recordUsage(userID, agentID string, tokens int) {
	if tokens <= 0 {
		return
	}
	go func() {
		_, err := g.db.Exec(`INSERT INTO ai_usage_stats (user_id, agent_id, total_tokens, request_count, period_start)
			VALUES ($1, $2, $3, 1, date_trunc('hour', NOW()))
			ON CONFLICT (user_id, agent_id, period_start)
			DO UPDATE SET total_tokens = ai_usage_stats.total_tokens + $3, request_count = ai_usage_stats.request_count + 1`,
			userID, agentID, tokens)
		if err != nil {
			logger.Warnf("recordUsage: %v", err)
		}
	}()
}

// GetAIUsageStats returns aggregated usage stats for a user
func (g *AIGateway) GetAIUsageStats(userID string) ([]*AIUsageStat, error) {
	rows, err := g.db.Query(`SELECT user_id, agent_id, total_tokens, request_count, period_start
		FROM ai_usage_stats WHERE user_id = $1::uuid ORDER BY period_start DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []*AIUsageStat
	for rows.Next() {
		var s AIUsageStat
		if err := rows.Scan(&s.UserID, &s.AgentID, &s.TotalTokens, &s.RequestCount, &s.PeriodStart); err != nil {
			return nil, err
		}
		stats = append(stats, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetAIUsageStatsSummary returns totals for a user
func (g *AIGateway) GetAIUsageStatsSummary(userID string) (totalTokens, totalRequests int, err error) {
	err = g.db.QueryRow(`SELECT COALESCE(SUM(total_tokens),0), COALESCE(SUM(request_count),0)
		FROM ai_usage_stats WHERE user_id = $1::uuid`, userID).Scan(&totalTokens, &totalRequests)
	return
}

// AIUsageStat represents a usage statistics record
type AIUsageStat struct {
	UserID       string    `json:"user_id"`
	AgentID      string    `json:"agent_id"`
	TotalTokens  int       `json:"total_tokens"`
	RequestCount int       `json:"request_count"`
	PeriodStart  time.Time `json:"period_start"`
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (g *AIGateway) dbAddMessage(m *AIMessageV2) error {
	_, err := g.db.Exec(`INSERT INTO ai_messages_v2 (chat_id, role, content, agent_id, token_count, model_used) VALUES ($1,$2,$3,$4,$5,$6)`,
		m.ChatID, m.Role, m.Content, m.AgentID, m.TokenCount, m.ModelUsed)
	return err
}
