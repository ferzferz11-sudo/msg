package main

// db_hermes.go — миграции и CRUD для Hermes Orchestrator
// Таблицы: hermes_messages, hermes_sessions, hermes_agent_runs

import (
	"database/sql"
	"log"
	"strings"
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
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "must be owner of table") {
				log.Printf("[Hermes] Migration error: %v", err)
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
		log.Printf("[HermesDB] save message error: %v", err)
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
		log.Printf("[HermesDB] save agent run error: %v", err)
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
		log.Printf("[HermesDB] complete agent run error: %v", err)
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
		log.Printf("[HermesDB] save session error: %v", err)
	}
}
