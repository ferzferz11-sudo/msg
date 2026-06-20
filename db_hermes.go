package main

// db_hermes.go — миграции и CRUD для Hermes Orchestrator
// Таблицы: hermes_messages, hermes_sessions, hermes_agent_runs

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"LavenderMessenger/auth"
)

// runHermesMigrations создаёт таблицы для Hermes Orchestrator
// Вызывается из ConnectDB() при старте сервера
func runHermesMigrations(db *sql.DB) {
	queries := []string{
		// Таблица сообщений оркестратора (история диалога)
		`CREATE TABLE IF NOT EXISTS hermes_messages (
			id BIGSERIAL PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL,
			user_id TEXT NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'agent')),
			agent_id TEXT DEFAULT '', -- ID агента (пусто = оркестратор)
			content TEXT NOT NULL DEFAULT '',
			is_streaming BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hermes_messages_session ON hermes_messages(session_id, created_at ASC)`,

		// Таблица сессий оркестратора
		`CREATE TABLE IF NOT EXISTS hermes_sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT DEFAULT '',
			active_agent_id TEXT DEFAULT '',
			agent_mode TEXT DEFAULT 'single', -- single, parallel, pipeline
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		// Миграция: agent_mode была добавлена позже
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='hermes_sessions' AND column_name='agent_mode'
			) THEN
				ALTER TABLE hermes_sessions ADD COLUMN agent_mode TEXT DEFAULT 'single';
			END IF;
		END$$;`,

		// Таблица запусков агентов (для аналитики и дебага)
		`CREATE TABLE IF NOT EXISTS hermes_agent_runs (
			id BIGSERIAL PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL,
			user_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			agent_mode TEXT DEFAULT 'single',
			routing_reason TEXT DEFAULT '',
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending', -- pending, running, completed, error
			error_text TEXT DEFAULT '',
			started_at TIMESTAMPTZ DEFAULT NOW(),
			completed_at TIMESTAMPTZ
		)`,

		// Добавляем creator_id в chats для хранения UUID создателя (creator_username ненадёжен — username может меняться)
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='chats' AND column_name='creator_id'
			) THEN
				ALTER TABLE chats ADD COLUMN creator_id TEXT DEFAULT '';
			END IF;
		END$$;`,

		// Добавляем agent_id в chats для поддержки hermes типа
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='chats' AND column_name='agent_id'
			) THEN
				ALTER TABLE chats ADD COLUMN agent_id TEXT DEFAULT '';
			END IF;
		END$$;`,

		// Таблица кастомных агентов пользователей
		`CREATE TABLE IF NOT EXISTS hermes_custom_agents (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			role VARCHAR(255) NOT NULL DEFAULT 'Custom',
			description TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			model VARCHAR(255) DEFAULT '',
			max_tokens INTEGER DEFAULT 2048,
			created_by TEXT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hermes_custom_agents_user ON hermes_custom_agents(created_by, is_active)`,

		// Миграция: добавляем недостающие колонки если отсутствуют
		`DO $$
		BEGIN
			ALTER TABLE hermes_custom_agents ADD COLUMN IF NOT EXISTS user_id TEXT;
			ALTER TABLE hermes_custom_agents ADD COLUMN IF NOT EXISTS preset_id TEXT;
			UPDATE hermes_custom_agents SET user_id = created_by WHERE user_id IS NULL;
		END$$;`,

		// Таблица удалённых агентов (реестр)
		`CREATE TABLE IF NOT EXISTS hermes_remote_agents (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			host VARCHAR(255) NOT NULL,
			ip_address VARCHAR(255),
			os VARCHAR(50),
			capabilities TEXT DEFAULT '[]', -- JSON array
			status VARCHAR(50) DEFAULT 'disconnected',
			registered_at TIMESTAMPTZ DEFAULT NOW(),
			last_heartbeat TIMESTAMPTZ
		)`,

		// Таблица задач удалённых агентов
		`CREATE TABLE IF NOT EXISTS hermes_remote_tasks (
			id VARCHAR(255) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			task_type VARCHAR(50) NOT NULL,
			params TEXT DEFAULT '{}', -- JSON
			status VARCHAR(50) DEFAULT 'pending',
			stdout TEXT DEFAULT '',
			stderr TEXT DEFAULT '',
			exit_code INTEGER DEFAULT 0,
			error TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			completed_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hermes_remote_tasks_agent ON hermes_remote_tasks(agent_id, status)`,

		// GRANT permissions for lavender user
		`GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO lavender`,
		`GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO lavender`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO lavender`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO lavender`,

		// === AI Chat Refactor v1.1.2.3 — unified tables ===
		// NOTE: Old DROP TABLE statements removed — CREATE TABLE IF NOT EXISTS handles idempotency safely

		// AI Chat Sessions — unified for OWL + Hermes
		`CREATE TABLE IF NOT EXISTS ai_chat_sessions (
			id              VARCHAR(255) PRIMARY KEY,
			user_id         TEXT        NOT NULL,
			agent_type      TEXT        NOT NULL DEFAULT 'owl',
			model           TEXT        DEFAULT '',
			system_prompt   TEXT        DEFAULT '',
			active_agent_id TEXT        DEFAULT '',
			agent_mode      TEXT        DEFAULT 'single',
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			updated_at      TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT fk_ai_sessions_chat
				FOREIGN KEY (id) REFERENCES chats(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_user ON ai_chat_sessions(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_type ON ai_chat_sessions(agent_type)`,

		// AI Chat Messages
		`CREATE TABLE IF NOT EXISTS ai_chat_messages (
			id              BIGSERIAL   PRIMARY KEY,
			session_id      VARCHAR(255) NOT NULL,
			role            TEXT        NOT NULL,
			content         TEXT        NOT NULL DEFAULT '',
			agent_id        TEXT        DEFAULT '',
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT fk_ai_messages_session
				FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_messages_session ON ai_chat_messages(session_id, created_at ASC)`,

		// AI Chat Settings
		`CREATE TABLE IF NOT EXISTS ai_chat_settings (
			session_id      VARCHAR(255) PRIMARY KEY,
			user_api_key    TEXT        DEFAULT '',
			model_override  TEXT        DEFAULT '',
			updated_at      TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT fk_ai_settings_session
				FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE
		)`,

		// Grant permissions for new tables
		`GRANT ALL PRIVILEGES ON ai_chat_sessions TO lavender`,
		`GRANT ALL PRIVILEGES ON ai_chat_messages TO lavender`,
		`GRANT ALL PRIVILEGES ON ai_chat_settings TO lavender`,
		`GRANT ALL PRIVILEGES ON ai_chat_messages_id_seq TO lavender`,

		// === Agent Auth Tokens (JWT) ===
		`CREATE TABLE IF NOT EXISTS agent_tokens (
			id              BIGSERIAL   PRIMARY KEY,
			agent_id        VARCHAR(255) NOT NULL,
			agent_name      VARCHAR(255) NOT NULL,
			token_hash      VARCHAR(255) NOT NULL UNIQUE, -- SHA-256 hash of token (we don't store raw tokens)
			capabilities    TEXT        DEFAULT '[]',     -- JSON array
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			expires_at      TIMESTAMPTZ,
			revoked         BOOLEAN     DEFAULT FALSE,
			created_by      TEXT        DEFAULT ''        -- admin user_id
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_tokens_agent ON agent_tokens(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_tokens_hash ON agent_tokens(token_hash)`,
		`GRANT ALL PRIVILEGES ON agent_tokens TO lavender`,
		`GRANT ALL PRIVILEGES ON agent_tokens_id_seq TO lavender`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "must be owner of table") {
				logger.Errorf("[Lava] Migration error: %v", err)
			}
		}
	}
}

// HermesDB — методы для работы с БД Hermes
type HermesDB struct {
	db *sql.DB
}

func NewHermesDB(db *sql.DB) *HermesDB {
	return &HermesDB{db: db}
}

// SaveOrchestratorMessage сохраняет сообщение оркестратора/агента
func (h *HermesDB) SaveOrchestratorMessage(sessionID, userID, role, agentID, content string) {
	_, err := h.db.Exec(
		`INSERT INTO hermes_messages (session_id, user_id, role, agent_id, content)
		 VALUES ($1, $2, $3, $4, $5)`,
		sessionID, userID, role, agentID, content,
	)
	if err != nil {
		logger.Errorf("[HermesDB] save message error: %v", err)
	}
}

// GetOrchestratorHistory возвращает историю сообщений сессии
func (h *HermesDB) GetOrchestratorHistory(sessionID string, limit int) []OrchestratorMessage {
	if limit <= 0 {
		limit = 50
	}

	rows, err := h.db.Query(
		`SELECT role, content FROM hermes_messages
		 WHERE session_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		sessionID, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var messages []OrchestratorMessage
	for rows.Next() {
		var msg OrchestratorMessage
		if err := rows.Scan(&msg.Role, &msg.Content); err == nil {
			messages = append(messages, msg)
		}
	}

	// Разворачиваем (DESC → ASC)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages
}

// SaveAgentRun сохраняет информацию о запуске агента
func (h *HermesDB) SaveAgentRun(sessionID, userID, agentID, mode, reason string) int {
	var id int
	err := h.db.QueryRow(
		`INSERT INTO hermes_agent_runs (session_id, user_id, agent_id, agent_mode, routing_reason, status)
		 VALUES ($1, $2, $3, $4, $5, 'running')
		 RETURNING id`,
		sessionID, userID, agentID, mode, reason,
	).Scan(&id)
	if err != nil {
		logger.Errorf("[HermesDB] save agent run error: %v", err)
		return 0
	}
	return id
}

// CompleteAgentRun обновляет статус запуска агента
func (h *HermesDB) CompleteAgentRun(runID int, status string) {
	_, err := h.db.Exec(
		`UPDATE hermes_agent_runs SET status = $1, completed_at = NOW() WHERE id = $2`,
		status, runID,
	)
	if err != nil {
		logger.Errorf("[HermesDB] complete agent run error: %v", err)
	}
}

// GetSessionActiveAgent возвращает активного агента сессии
func (h *HermesDB) GetSessionActiveAgent(sessionID string) string {
	var agentID string
	err := h.db.QueryRow(
		"SELECT active_agent_id FROM hermes_sessions WHERE id = $1", sessionID,
	).Scan(&agentID)
	if err != nil {
		return ""
	}
	return agentID
}

// GetUserHermesSessions возвращает список hermes-сессий пользователя
func (h *HermesDB) GetUserHermesSessions(userID string) ([]struct {
	ID              string
	Name            string
	ActiveAgentID   string
	AgentMode       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastMessageText string
	LastMessageTime time.Time
}, error) {
	query := `
		SELECT s.id, s.name, s.active_agent_id, s.agent_mode, s.created_at, s.updated_at,
		       COALESCE(m.content, '') as last_message_text,
		       COALESCE(m.created_at, s.updated_at) as last_message_time
		FROM hermes_sessions s
		LEFT JOIN LATERAL (
			SELECT content, created_at FROM hermes_messages
			WHERE session_id = s.id AND role = 'assistant'
			ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE s.user_id = $1
		ORDER BY s.updated_at DESC`
	rows, err := h.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []struct {
		ID              string
		Name            string
		ActiveAgentID   string
		AgentMode       string
		CreatedAt       time.Time
		UpdatedAt       time.Time
		LastMessageText string
		LastMessageTime time.Time
	}
	for rows.Next() {
		var r struct {
			ID              string
			Name            string
			ActiveAgentID   string
			AgentMode       string
			CreatedAt       time.Time
			UpdatedAt       time.Time
			LastMessageText string
			LastMessageTime time.Time
		}
		if err := rows.Scan(&r.ID, &r.Name, &r.ActiveAgentID, &r.AgentMode, &r.CreatedAt, &r.UpdatedAt, &r.LastMessageText, &r.LastMessageTime); err != nil {
			logger.Errorf("[HermesDB] GetUserHermesSessions scan error: %v", err)
			continue
		}
		res = append(res, r)
	}
	return res, nil
}

// SaveSession сохраняет или обновляет сессию
func (h *HermesDB) SaveSession(sessionID, userID, activeAgentID, mode string) {
	_, err := h.db.Exec(
		`INSERT INTO hermes_sessions (id, user_id, active_agent_id, agent_mode, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		 active_agent_id = $3, agent_mode = $4, updated_at = NOW()`,
		sessionID, userID, activeAgentID, mode,
	)
	if err != nil {
		logger.Errorf("[HermesDB] save session error: %v", err)
	}
}

// GetSessionID checks if a hermes session exists by ID (returns session ID or empty string)
func (h *HermesDB) GetSessionID(sessionID string) string {
	var id string
	err := h.db.QueryRow("SELECT id FROM hermes_sessions WHERE id = $1", sessionID).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// DeleteSession removes a hermes session by ID
func (h *HermesDB) DeleteSession(sessionID string) error {
	_, err := h.db.Exec("DELETE FROM hermes_sessions WHERE id = $1", sessionID)
	return err
}

// === Agent Token Methods ===

// SaveAgentToken сохраняет хеш токена агента в БД
func (h *HermesDB) SaveAgentToken(agentID, agentName, tokenHash string, capabilities []string, expiresAt time.Time, createdBy string) error {
	capsJSON, _ := json.Marshal(capabilities)
	_, err := h.db.Exec(
		`INSERT INTO agent_tokens (agent_id, agent_name, token_hash, capabilities, expires_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (token_hash) DO UPDATE SET
		 agent_name = $2, capabilities = $4, expires_at = $5, revoked = FALSE`,
		agentID, agentName, tokenHash, string(capsJSON), expiresAt, createdBy,
	)
	return err
}

// GetAgentTokenByHash возвращает токен по хешу
func (h *HermesDB) GetAgentTokenByHash(tokenHash string) (*auth.AgentToken, error) {
	var t auth.AgentToken
	var capsStr string
	err := h.db.QueryRow(
		`SELECT id, agent_id, agent_name, token_hash, capabilities, created_at, expires_at, revoked, created_by
		 FROM agent_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&t.ID, &t.AgentID, &t.AgentName, &t.Token, &capsStr, &t.CreatedAt, &t.ExpiresAt, &t.Revoked, &t.CreatedBy)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(capsStr), &t.Capabilities)
	return &t, nil
}

// RevokeAgentToken отзывает токен агента
func (h *HermesDB) RevokeAgentToken(agentID string) error {
	_, err := h.db.Exec("UPDATE agent_tokens SET revoked = TRUE WHERE agent_id = $1", agentID)
	return err
}

// ListAgentTokens возвращает все токены агентов
func (h *HermesDB) ListAgentTokens() ([]auth.AgentToken, error) {
	return h.ListAgentTokensFiltered("")
}

// ListAgentTokensFiltered возвращает токены агентов, отфильтрованные по created_by
// Если createdBy пусто — возвращает все токены (для админов)
func (h *HermesDB) ListAgentTokensFiltered(createdBy string) ([]auth.AgentToken, error) {
	var rows *sql.Rows
	var err error

	if createdBy != "" {
		rows, err = h.db.Query(
			`SELECT id, agent_id, agent_name, token_hash, capabilities, created_at, expires_at, revoked, created_by
			 FROM agent_tokens WHERE created_by = $1 ORDER BY created_at DESC`,
			createdBy,
		)
	} else {
		rows, err = h.db.Query(
			`SELECT id, agent_id, agent_name, token_hash, capabilities, created_at, expires_at, revoked, created_by
			 FROM agent_tokens ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []auth.AgentToken
	for rows.Next() {
		var t auth.AgentToken
		var capsStr string
		var createdBy string
		if err := rows.Scan(&t.ID, &t.AgentID, &t.AgentName, &t.Token, &capsStr, &t.CreatedAt, &t.ExpiresAt, &t.Revoked, &createdBy); err != nil {
			continue
		}
		t.CreatedBy = createdBy
		json.Unmarshal([]byte(capsStr), &t.Capabilities)
		tokens = append(tokens, t)
	}
	return tokens, nil
}
