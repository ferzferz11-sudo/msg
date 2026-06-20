package main

// db_ai_v2.go — AI Services v2: DB layer, migrations, CRUD
// Replaces: ai_chat_manager.go, hermes_agents.go, owl.go session management

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ======= Types =======

type AgentV2 struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	ProviderType    string         `json:"provider_type"`
	ProviderConfig  map[string]any `json:"provider_config"`
	SystemPrompt    string         `json:"system_prompt"`
	Model           string         `json:"model"`
	MaxTokens       int            `json:"max_tokens"`
	Temperature     float64        `json:"temperature"`
	ToolsEnabled    bool           `json:"tools_enabled"`
	ToolWhitelist   []string       `json:"tool_whitelist"`
	RAGEnabled      bool           `json:"rag_enabled"`
	RAGConfig       map[string]any `json:"rag_config"`
	RateLimit       *int           `json:"rate_limit"`
	IsPreset        bool           `json:"is_preset"`
	IsPublic        bool           `json:"is_public"`
	IsActive        bool           `json:"is_active"`
	CreatedBy       string         `json:"created_by"`
	InstallCount    int            `json:"install_count"`
	AvgRating       float64        `json:"avg_rating"`
	ReviewCount     int            `json:"review_count"`
	Tags            []string       `json:"tags"`
	OriginalAgentID string         `json:"original_agent_id"`
	Version         int            `json:"version"`
	ShareCode       string         `json:"share_code"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type AIChatV2 struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	ChatType      string         `json:"chat_type"`
	Name          string         `json:"name"`
	AgentID       string         `json:"agent_id"`
	Model         string         `json:"model"`
	SystemPrompt  string         `json:"system_prompt"`
	BoundAgentID  string         `json:"bound_agent_id"`
	BindUntilMsg  int            `json:"bind_until_msg"`
	Settings      map[string]any `json:"settings"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type AIMessageV2 struct {
	ID          int64            `json:"id"`
	ChatID      string           `json:"chat_id"`
	Role        string           `json:"role"`
	Content     string           `json:"content"`
	AgentID     string           `json:"agent_id"`
	ToolCalls   []ToolCallResult `json:"tool_calls"`
	ToolResults []ToolCallResult `json:"tool_results"`
	Images      [][]byte         `json:"images"`
	TokenCount  int              `json:"token_count"`
	ModelUsed   string           `json:"model_used"`
	CreatedAt   time.Time        `json:"created_at"`
}

type ToolCallResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

// ======= Migrations =======

// DropOldAIV1 removes all v1 AI tables and data
func DropOldAIV1(db *sql.DB) {
	queries := []string{
		`DROP TABLE IF EXISTS ai_chat_messages CASCADE`,
		`DROP TABLE IF EXISTS ai_chat_settings CASCADE`,
		`DROP TABLE IF EXISTS ai_chat_sessions CASCADE`,
		`DROP TABLE IF EXISTS owl_messages CASCADE`,
		`DROP TABLE IF EXISTS owl_chat_settings CASCADE`,
		`DROP TABLE IF EXISTS hermes_chat_settings CASCADE`,
		`DROP TABLE IF EXISTS hermes_messages CASCADE`,
		`DROP TABLE IF EXISTS hermes_sessions CASCADE`,
		`DROP TABLE IF EXISTS hermes_custom_agents CASCADE`,
		`DROP TABLE IF EXISTS hermes_agent_runs CASCADE`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			logger.Warnf("DropOldAIV1: %v", err)
		}
	}
	logger.Info("Old AI v1 tables dropped")
}

// MigrateAIV2 creates all v2 AI tables and seeds preset agents
func MigrateAIV2(db *sql.DB) error {
	queries := []string{
		// Agents v2
		`CREATE TABLE IF NOT EXISTS agents_v2 (
			id              VARCHAR(255) PRIMARY KEY,
			name            VARCHAR(255) NOT NULL,
			description     TEXT DEFAULT '',
			provider_type   VARCHAR(50) NOT NULL,
			provider_config JSONB NOT NULL DEFAULT '{}',
			system_prompt   TEXT DEFAULT '',
			model           VARCHAR(255) DEFAULT '',
			max_tokens      INT DEFAULT 4096,
			temperature     FLOAT DEFAULT 0.7,
			tools_enabled   BOOLEAN DEFAULT FALSE,
			tool_whitelist  TEXT[],
			rag_enabled     BOOLEAN DEFAULT FALSE,
			rag_config      JSONB DEFAULT '{}',
			rate_limit      INT,
			is_preset       BOOLEAN DEFAULT FALSE,
			is_public       BOOLEAN DEFAULT FALSE,
			is_active       BOOLEAN DEFAULT TRUE,
			created_by      UUID REFERENCES users(id),
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			updated_at      TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_v2_creator ON agents_v2(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_v2_active ON agents_v2(is_active) WHERE is_active = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_agents_v2_public ON agents_v2(is_public) WHERE is_public = TRUE AND is_active = TRUE`,

		// AI chats v2
		`CREATE TABLE IF NOT EXISTS ai_chats_v2 (
			id              VARCHAR(255) PRIMARY KEY,
			user_id         UUID NOT NULL REFERENCES users(id),
			chat_type       VARCHAR(20) NOT NULL,
			name            VARCHAR(255) DEFAULT '',
			agent_id        VARCHAR(255) DEFAULT '',
			model           VARCHAR(255) DEFAULT '',
			system_prompt   TEXT DEFAULT '',
			bound_agent_id  VARCHAR(255) DEFAULT '',
			bind_until_msg  INT DEFAULT 0,
			settings        JSONB DEFAULT '{}',
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			updated_at      TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_chats_v2_user ON ai_chats_v2(user_id, created_at DESC)`,

		// AI messages v2
		`CREATE TABLE IF NOT EXISTS ai_messages_v2 (
			id              BIGSERIAL PRIMARY KEY,
			chat_id         VARCHAR(255) NOT NULL REFERENCES ai_chats_v2(id) ON DELETE CASCADE,
			role            VARCHAR(20) NOT NULL,
			content         TEXT DEFAULT '',
			agent_id        VARCHAR(255) DEFAULT '',
			tool_calls      JSONB,
			tool_results    JSONB,
			images          BYTEA[],
			token_count     INT DEFAULT 0,
			model_used      VARCHAR(255) DEFAULT '',
			created_at      TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_messages_v2_chat ON ai_messages_v2(chat_id, created_at ASC)`,

		// Rate limits per agent
		`CREATE TABLE IF NOT EXISTS ai_rate_limits (
			agent_id            VARCHAR(255) PRIMARY KEY,
			requests_per_minute INT DEFAULT 10,
			requests_per_hour   INT DEFAULT 100,
			tokens_per_minute   INT DEFAULT 100000
		)`,

		// Usage stats (aggregated per user/agent/hour)
		`CREATE TABLE IF NOT EXISTS ai_usage_stats (
			user_id       UUID NOT NULL REFERENCES users(id),
			agent_id      VARCHAR(255) NOT NULL,
			total_tokens  INT DEFAULT 0,
			request_count INT DEFAULT 0,
			period_start  TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (user_id, agent_id, period_start)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_usage_stats_user ON ai_usage_stats(user_id, period_start DESC)`,

		// Agent reviews (marketplace)
		`CREATE TABLE IF NOT EXISTS agent_reviews (
			id          BIGSERIAL PRIMARY KEY,
			agent_id    VARCHAR(255) NOT NULL REFERENCES agents_v2(id) ON DELETE CASCADE,
			user_id     UUID NOT NULL REFERENCES users(id),
			rating      INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
			review      TEXT DEFAULT '',
			created_at  TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (agent_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_reviews_agent ON agent_reviews(agent_id)`,

		// Agent marketplace fields (added to agents_v2)
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS install_count INT DEFAULT 0`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS avg_rating FLOAT DEFAULT 0`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS review_count INT DEFAULT 0`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}'`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS original_agent_id VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS version INT DEFAULT 1`,
		`ALTER TABLE agents_v2 ADD COLUMN IF NOT EXISTS share_code VARCHAR(32) DEFAULT ''`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %v", err)
		}
	}

	// Seed preset agents
	if err := seedPresetAgents(db); err != nil {
		logger.Warnf("seedPresetAgents: %v", err)
	}

	logger.Info("AI v2 tables ready")
	return nil
}

func seedPresetAgents(db *sql.DB) error {
	presets := []struct {
		id, name, desc, provider, model, prompt string
		tools, rag                              bool
	}{
		{"mimo", "MiMo", "AI assistant integrated into Lavender Messenger", "mimo", "mimo-auto", "You are MiMo, an AI assistant integrated into Lavender Messenger. You help users with their tasks, answer questions, and use available tools when needed.", true, true},
		{"assistant", "Assistant", "Universal AI assistant", "openrouter", "anthropic/claude-sonnet-4", "You are a helpful AI assistant. Be concise, accurate, and helpful.", true, true},
		{"developer", "Developer", "Code writing, refactoring, debugging", "openrouter", "anthropic/claude-sonnet-4", "You are an expert software developer. Help with code writing, refactoring, debugging, and code review. Always provide clean, production-ready code.", true, false},
		{"devops", "DevOps", "Server management, deploy, monitoring", "openrouter", "anthropic/claude-sonnet-4", "You are a DevOps engineer. Help with server management, deployment, CI/CD, monitoring, and infrastructure.", true, false},
		{"architect", "Architect", "System design, architecture decisions", "openrouter", "anthropic/claude-sonnet-4", "You are a system architect. Help with system design, architecture decisions, trade-offs, and scalability.", false, false},
		{"writer", "Writer", "Creative writing, content creation", "openrouter", "openai/gpt-4o", "You are a creative writer. Help with writing, editing, content creation, and storytelling.", false, false},
		{"analyst", "Analyst", "Data analysis, metrics, reports", "openrouter", "anthropic/claude-sonnet-4", "You are a data analyst. Help with data analysis, metrics interpretation, report generation, and insights.", true, true},
		{"translator", "Translator", "Multi-language translation", "openrouter", "openai/gpt-4o-mini", "You are a professional translator. Translate text accurately between languages, preserving meaning and tone.", false, false},
	}

	for _, p := range presets {
		pc := map[string]any{"api_key_source": "user", "default_model": p.model}
		pcJSON, _ := json.Marshal(pc)
		query := `INSERT INTO agents_v2 (id, name, description, provider_type, provider_config, system_prompt, model, tools_enabled, rag_enabled, is_preset, is_public, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, TRUE, TRUE)
			ON CONFLICT (id) DO NOTHING`
		if _, err := db.Exec(query, p.id, p.name, p.desc, p.provider, string(pcJSON), p.prompt, p.model, p.tools, p.rag); err != nil {
			return fmt.Errorf("seed %s: %v", p.id, err)
		}
	}
	logger.Infof("Seeded %d preset agents", len(presets))
	return nil
}

// ======= Agents CRUD =======

func (d *DB) CreateAgentV2(a *AgentV2) error {
	pcJSON, err := json.Marshal(a.ProviderConfig)
	if err != nil {
		return err
	}
	rcJSON, err := json.Marshal(a.RAGConfig)
	if err != nil {
		return err
	}
	query := `INSERT INTO agents_v2 (id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`
	_, err = d.Exec(query, a.ID, a.Name, a.Description, a.ProviderType, string(pcJSON), a.SystemPrompt, a.Model, a.MaxTokens, a.Temperature, a.ToolsEnabled, arrayToNullable(a.ToolWhitelist), a.RAGEnabled, string(rcJSON), a.RateLimit, a.IsPreset, a.IsPublic, a.IsActive, a.CreatedBy)
	return err
}

func (d *DB) GetAgentV2(id string) (*AgentV2, error) {
	var a AgentV2
	var pcJSON, rcJSON string
	var toolWL []string
	query := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE id = $1 AND is_active = TRUE`
	err := d.QueryRow(query, id).Scan(&a.ID, &a.Name, &a.Description, &a.ProviderType, &pcJSON, &a.SystemPrompt, &a.Model, &a.MaxTokens, &a.Temperature, &a.ToolsEnabled, &toolWL, &a.RAGEnabled, &rcJSON, &a.RateLimit, &a.IsPreset, &a.IsPublic, &a.IsActive, &a.CreatedBy, &a.InstallCount, &a.AvgRating, &a.ReviewCount, &a.Tags, &a.OriginalAgentID, &a.Version, &a.ShareCode, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(pcJSON), &a.ProviderConfig)
	json.Unmarshal([]byte(rcJSON), &a.RAGConfig)
	a.ToolWhitelist = toolWL
	return &a, nil
}

func (d *DB) ListAgentsV2(userID string, includePublic bool) ([]*AgentV2, error) {
	var query string
	var args []any
	if includePublic {
		query = `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
			FROM agents_v2 WHERE is_active = TRUE AND (created_by = $1::uuid OR is_public = TRUE) ORDER BY is_preset DESC, name ASC`
		args = []any{userID}
	} else {
		query = `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
			FROM agents_v2 WHERE is_active = TRUE AND created_by = $1::uuid ORDER BY name ASC`
		args = []any{userID}
	}
	return d.scanAgents(query, args...)
}

func (d *DB) ListPresetAgentsV2() ([]*AgentV2, error) {
	query := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE is_active = TRUE AND is_preset = TRUE ORDER BY name ASC`
	return d.scanAgents(query)
}

func (d *DB) ListAllActiveAgentsV2() ([]*AgentV2, error) {
	query := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE is_active = TRUE ORDER BY is_preset DESC, name ASC`
	return d.scanAgents(query)
}

func (d *DB) UpdateAgentV2(a *AgentV2) error {
	pcJSON, err := json.Marshal(a.ProviderConfig)
	if err != nil {
		return err
	}
	rcJSON, err := json.Marshal(a.RAGConfig)
	if err != nil {
		return err
	}
	query := `UPDATE agents_v2 SET name=$2, description=$3, provider_config=$4, system_prompt=$5, model=$6, max_tokens=$7, temperature=$8, tools_enabled=$9, tool_whitelist=$10, rag_enabled=$11, rag_config=$12, rate_limit=$13, is_public=$14, updated_at=NOW()
		WHERE id = $1`
	_, err = d.Exec(query, a.ID, a.Name, a.Description, string(pcJSON), a.SystemPrompt, a.Model, a.MaxTokens, a.Temperature, a.ToolsEnabled, arrayToNullable(a.ToolWhitelist), a.RAGEnabled, string(rcJSON), a.RateLimit, a.IsPublic)
	return err
}

func (d *DB) DeleteAgentV2(id string) error {
	_, err := d.Exec(`UPDATE agents_v2 SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (d *DB) scanAgents(query string, args ...any) ([]*AgentV2, error) {
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []*AgentV2
	for rows.Next() {
		var a AgentV2
		var pcJSON, rcJSON string
		var toolWL []string
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.ProviderType, &pcJSON, &a.SystemPrompt, &a.Model, &a.MaxTokens, &a.Temperature, &a.ToolsEnabled, &toolWL, &a.RAGEnabled, &rcJSON, &a.RateLimit, &a.IsPreset, &a.IsPublic, &a.IsActive, &a.CreatedBy, &a.InstallCount, &a.AvgRating, &a.ReviewCount, &a.Tags, &a.OriginalAgentID, &a.Version, &a.ShareCode, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.ProviderConfig = make(map[string]any)
		json.Unmarshal([]byte(pcJSON), &a.ProviderConfig)
		a.RAGConfig = make(map[string]any)
		json.Unmarshal([]byte(rcJSON), &a.RAGConfig)
		a.ToolWhitelist = toolWL
		agents = append(agents, &a)
	}
	return agents, nil
}

// ======= AI Chats CRUD =======

func (d *DB) CreateAIChatV2(c *AIChatV2) error {
	sJSON, err := json.Marshal(c.Settings)
	if err != nil {
		return err
	}
	query := `INSERT INTO ai_chats_v2 (id, user_id, chat_type, name, agent_id, model, system_prompt, bound_agent_id, bind_until_msg, settings)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err = d.Exec(query, c.ID, c.UserID, c.ChatType, c.Name, c.AgentID, c.Model, c.SystemPrompt, c.BoundAgentID, c.BindUntilMsg, string(sJSON))
	return err
}

func (d *DB) GetAIChatV2(id string) (*AIChatV2, error) {
	var c AIChatV2
	var sJSON string
	query := `SELECT id, user_id, chat_type, name, agent_id, model, system_prompt, bound_agent_id, bind_until_msg, settings, created_at, updated_at
		FROM ai_chats_v2 WHERE id = $1`
	err := d.QueryRow(query, id).Scan(&c.ID, &c.UserID, &c.ChatType, &c.Name, &c.AgentID, &c.Model, &c.SystemPrompt, &c.BoundAgentID, &c.BindUntilMsg, &sJSON, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Settings = make(map[string]any)
	json.Unmarshal([]byte(sJSON), &c.Settings)
	return &c, nil
}

func (d *DB) ListAIChatsV2(userID string) ([]*AIChatV2, error) {
	query := `SELECT id, user_id, chat_type, name, agent_id, model, system_prompt, bound_agent_id, bind_until_msg, settings, created_at, updated_at
		FROM ai_chats_v2 WHERE user_id = $1::uuid ORDER BY updated_at DESC`
	rows, err := d.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chats []*AIChatV2
	for rows.Next() {
		var c AIChatV2
		var sJSON string
		if err := rows.Scan(&c.ID, &c.UserID, &c.ChatType, &c.Name, &c.AgentID, &c.Model, &c.SystemPrompt, &c.BoundAgentID, &c.BindUntilMsg, &sJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Settings = make(map[string]any)
		json.Unmarshal([]byte(sJSON), &c.Settings)
		chats = append(chats, &c)
	}
	return chats, nil
}

func (d *DB) UpdateAIChatV2(c *AIChatV2) error {
	sJSON, err := json.Marshal(c.Settings)
	if err != nil {
		return err
	}
	query := `UPDATE ai_chats_v2 SET name=$2, agent_id=$3, model=$4, system_prompt=$5, bound_agent_id=$6, bind_until_msg=$7, settings=$8, updated_at=NOW()
		WHERE id = $1`
	_, err = d.Exec(query, c.ID, c.Name, c.AgentID, c.Model, c.SystemPrompt, c.BoundAgentID, c.BindUntilMsg, string(sJSON))
	return err
}

func (d *DB) DeleteAIChatV2(id string) error {
	_, err := d.Exec(`DELETE FROM ai_chats_v2 WHERE id = $1`, id)
	return err
}

// ======= AI Messages CRUD =======

// GetAgentV2FromDB is a standalone function for getting an agent (used by gateway)
func GetAgentV2FromDB(d *sql.DB, id string) (*AgentV2, error) {
	var a AgentV2
	var pcJSON, rcJSON string
	var toolWL []string
	query := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE id = $1 AND is_active = TRUE`
	err := d.QueryRow(query, id).Scan(&a.ID, &a.Name, &a.Description, &a.ProviderType, &pcJSON, &a.SystemPrompt, &a.Model, &a.MaxTokens, &a.Temperature, &a.ToolsEnabled, &toolWL, &a.RAGEnabled, &rcJSON, &a.RateLimit, &a.IsPreset, &a.IsPublic, &a.IsActive, &a.CreatedBy, &a.InstallCount, &a.AvgRating, &a.ReviewCount, &a.Tags, &a.OriginalAgentID, &a.Version, &a.ShareCode, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(pcJSON), &a.ProviderConfig)
	json.Unmarshal([]byte(rcJSON), &a.RAGConfig)
	a.ToolWhitelist = toolWL
	return &a, nil
}

func (d *DB) AddAIMessageV2(m *AIMessageV2) error {
	var tcJSON, trJSON *string
	if m.ToolCalls != nil {
		b, _ := json.Marshal(m.ToolCalls)
		s := string(b)
		tcJSON = &s
	}
	if m.ToolResults != nil {
		b, _ := json.Marshal(m.ToolResults)
		s := string(b)
		trJSON = &s
	}
	query := `INSERT INTO ai_messages_v2 (chat_id, role, content, agent_id, tool_calls, tool_results, token_count, model_used)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := d.Exec(query, m.ChatID, m.Role, m.Content, m.AgentID, tcJSON, trJSON, m.TokenCount, m.ModelUsed)
	return err
}

func (d *DB) GetAIMessagesV2(chatID string, limit int) ([]*AIMessageV2, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, chat_id, role, content, agent_id, tool_calls, tool_results, token_count, model_used, created_at
		FROM ai_messages_v2 WHERE chat_id = $1 ORDER BY created_at ASC LIMIT $2`
	rows, err := d.Query(query, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*AIMessageV2
	for rows.Next() {
		var m AIMessageV2
		var tcJSON, trJSON sql.NullString
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.AgentID, &tcJSON, &trJSON, &m.TokenCount, &m.ModelUsed, &m.CreatedAt); err != nil {
			return nil, err
		}
		if tcJSON.Valid {
			json.Unmarshal([]byte(tcJSON.String), &m.ToolCalls)
		}
		if trJSON.Valid {
			json.Unmarshal([]byte(trJSON.String), &m.ToolResults)
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

// ======= Helpers =======

func arrayToNullable(arr []string) interface{} {
	if arr == nil {
		return nil
	}
	if len(arr) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

// ======= Agent Reviews (Marketplace) =======

type AgentReview struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	UserID    string    `json:"user_id"`
	Rating    int       `json:"rating"`
	Review    string    `json:"review"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *DB) AddAgentReview(r *AgentReview) error {
	_, err := d.Exec(`INSERT INTO agent_reviews (agent_id, user_id, rating, review) VALUES ($1, $2, $3, $4)
		ON CONFLICT (agent_id, user_id) DO UPDATE SET rating = $3, review = $4`,
		r.AgentID, r.UserID, r.Rating, r.Review)
	if err != nil {
		return err
	}
	// Update agent rating aggregates
	_, err = d.Exec(`UPDATE agents_v2 SET avg_rating = (SELECT COALESCE(AVG(rating),0) FROM agent_reviews WHERE agent_id = $1),
		review_count = (SELECT COUNT(*) FROM agent_reviews WHERE agent_id = $1) WHERE id = $1`, r.AgentID)
	return err
}

func (d *DB) GetAgentReviews(agentID string, limit int) ([]*AgentReview, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.Query(`SELECT id, agent_id, user_id, rating, review, created_at
		FROM agent_reviews WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reviews []*AgentReview
	for rows.Next() {
		var r AgentReview
		if err := rows.Scan(&r.ID, &r.AgentID, &r.UserID, &r.Rating, &r.Review, &r.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, &r)
	}
	return reviews, nil
}

func (d *DB) DeleteAgentReview(agentID, userID string) error {
	_, err := d.Exec(`DELETE FROM agent_reviews WHERE agent_id = $1 AND user_id = $2`, agentID, userID)
	if err != nil {
		return err
	}
	_, err = d.Exec(`UPDATE agents_v2 SET avg_rating = (SELECT COALESCE(AVG(rating),0) FROM agent_reviews WHERE agent_id = $1),
		review_count = (SELECT COUNT(*) FROM agent_reviews WHERE agent_id = $1) WHERE id = $1`, agentID)
	return err
}

func (d *DB) IncrementInstallCount(agentID string) error {
	_, err := d.Exec(`UPDATE agents_v2 SET install_count = install_count + 1 WHERE id = $1`, agentID)
	return err
}

func (d *DB) ListMarketplaceAgents(query string, limit, offset int) ([]*AgentV2, error) {
	if limit <= 0 {
		limit = 20
	}
	sqlQuery := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE is_active = TRUE AND is_public = TRUE`
	var args []any
	if query != "" {
		sqlQuery += ` AND (name ILIKE $1 OR description ILIKE $1 OR $2 = ANY(tags))`
		args = append(args, "%"+query+"%", query)
		sqlQuery += ` ORDER BY avg_rating DESC, install_count DESC LIMIT $3 OFFSET $4`
		args = append(args, limit, offset)
	} else {
		sqlQuery += ` ORDER BY avg_rating DESC, install_count DESC LIMIT $1 OFFSET $2`
		args = append(args, limit, offset)
	}
	return d.scanAgents(sqlQuery, args...)
}

func (d *DB) GetAgentByShareCode(shareCode string) (*AgentV2, error) {
	var a AgentV2
	var pcJSON, rcJSON string
	var toolWL []string
	query := `SELECT id, name, description, provider_type, provider_config, system_prompt, model, max_tokens, temperature, tools_enabled, tool_whitelist, rag_enabled, rag_config, rate_limit, is_preset, is_public, is_active, COALESCE(created_by,''), install_count, avg_rating, review_count, tags, original_agent_id, version, share_code, created_at, updated_at
		FROM agents_v2 WHERE share_code = $1 AND is_active = TRUE`
	err := d.QueryRow(query, shareCode).Scan(&a.ID, &a.Name, &a.Description, &a.ProviderType, &pcJSON, &a.SystemPrompt, &a.Model, &a.MaxTokens, &a.Temperature, &a.ToolsEnabled, &toolWL, &a.RAGEnabled, &rcJSON, &a.RateLimit, &a.IsPreset, &a.IsPublic, &a.IsActive, &a.CreatedBy, &a.InstallCount, &a.AvgRating, &a.ReviewCount, &a.Tags, &a.OriginalAgentID, &a.Version, &a.ShareCode, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(pcJSON), &a.ProviderConfig)
	json.Unmarshal([]byte(rcJSON), &a.RAGConfig)
	a.ToolWhitelist = toolWL
	return &a, nil
}

func (d *DB) SetAgentShareCode(agentID, shareCode string) error {
	_, err := d.Exec(`UPDATE agents_v2 SET share_code = $2 WHERE id = $1`, agentID, shareCode)
	return err
}
