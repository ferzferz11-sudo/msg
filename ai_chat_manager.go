package main

// ai_chat_manager.go — unified AI chat manager for OWL and Hermes
// Replaces owlSessionManager, hermesSettingsManager, and hermes_sessions table

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// AIChatSession represents a unified AI chat session (OWL or Hermes)
type AIChatSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	AgentType      string    `json:"agent_type"` // "owl" | "hermes"
	Model          string    `json:"model"`
	SystemPrompt   string    `json:"system_prompt"`
	ActiveAgentID  string    `json:"active_agent_id"`
	AgentMode      string    `json:"agent_mode"` // "single" | "parallel" | "pipeline"
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AIMessage represents a single message in an AI chat
type AIMessage struct {
	ID        int       `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // "user" | "assistant" | "system" | "agent"
	Content   string    `json:"content"`
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AIChatSettings represents per-chat settings (API key, model override)
type AIChatSettings struct {
	SessionID     string    `json:"session_id"`
	UserAPIKey    string    `json:"user_api_key"`
	ModelOverride string    `json:"model_override"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AIChatManager provides unified CRUD for AI chat sessions, messages, and settings
type AIChatManager struct {
	db *sql.DB
}

// NewAIChatManager creates a new AIChatManager
func NewAIChatManager(db *sql.DB) *AIChatManager {
	return &AIChatManager{db: db}
}

// CreateSession creates a new AI chat session and returns its ID
func (m *AIChatManager) CreateSession(userID, agentType string) (string, error) {
	if agentType == "" {
		agentType = "owl"
	}
	sessionID := agentType + "-" + uuid.New().String()

	_, err := m.db.Exec(
		`INSERT INTO ai_chat_sessions (id, user_id, agent_type)
		 VALUES ($1, $2, $3)`,
		sessionID, userID, agentType,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create AI chat session: %w", err)
	}
	log.Printf("[AIChatManager] created session %s for user %s (type=%s)", sessionID, userID, agentType)
	return sessionID, nil
}

// GetSession retrieves an AI chat session by ID
func (m *AIChatManager) GetSession(sessionID string) (*AIChatSession, error) {
	var s AIChatSession
	err := m.db.QueryRow(
		`SELECT id, user_id, agent_type, COALESCE(model, ''), COALESCE(system_prompt, ''),
		        COALESCE(active_agent_id, ''), COALESCE(agent_mode, 'single'),
		        created_at, updated_at
		 FROM ai_chat_sessions WHERE id = $1`,
		sessionID,
	).Scan(&s.ID, &s.UserID, &s.AgentType, &s.Model, &s.SystemPrompt,
		&s.ActiveAgentID, &s.AgentMode, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get session %s: %w", sessionID, err)
	}
	return &s, nil
}

// GetSessionsByUser returns all AI chat sessions for a user
func (m *AIChatManager) GetSessionsByUser(userID string) ([]*AIChatSession, error) {
	rows, err := m.db.Query(
		`SELECT id, user_id, agent_type, COALESCE(model, ''), COALESCE(system_prompt, ''),
		        COALESCE(active_agent_id, ''), COALESCE(agent_mode, 'single'),
		        created_at, updated_at
		 FROM ai_chat_sessions WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions for user %s: %w", userID, err)
	}
	defer rows.Close()

	var sessions []*AIChatSession
	for rows.Next() {
		var s AIChatSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.AgentType, &s.Model, &s.SystemPrompt,
			&s.ActiveAgentID, &s.AgentMode, &s.CreatedAt, &s.UpdatedAt); err != nil {
			log.Printf("[AIChatManager] scan error: %v", err)
			continue
		}
		sessions = append(sessions, &s)
	}
	return sessions, nil
}

// DeleteSession deletes an AI chat session (cascades to messages and settings via FK)
func (m *AIChatManager) DeleteSession(sessionID string) error {
	_, err := m.db.Exec("DELETE FROM ai_chat_sessions WHERE id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session %s: %w", sessionID, err)
	}
	log.Printf("[AIChatManager] deleted session %s", sessionID)
	return nil
}

// AddMessage adds a message to a session
func (m *AIChatManager) AddMessage(sessionID, role, content, agentID string) error {
	_, err := m.db.Exec(
		`INSERT INTO ai_chat_messages (session_id, role, content, agent_id)
		 VALUES ($1, $2, $3, $4)`,
		sessionID, role, content, agentID,
	)
	if err != nil {
		return fmt.Errorf("failed to add message to session %s: %w", sessionID, err)
	}
	return nil
}

// GetHistory returns message history for a session
func (m *AIChatManager) GetHistory(sessionID string, limit int) ([]AIMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := m.db.Query(
		`SELECT id, session_id, role, content, COALESCE(agent_id, ''), created_at
		 FROM ai_chat_messages
		 WHERE session_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get history for session %s: %w", sessionID, err)
	}
	defer rows.Close()

	var messages []AIMessage
	for rows.Next() {
		var msg AIMessage
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.AgentID, &msg.CreatedAt); err != nil {
			log.Printf("[AIChatManager] history scan error: %v", err)
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetSettings returns per-chat settings
func (m *AIChatManager) GetSettings(sessionID string) (*AIChatSettings, error) {
	var s AIChatSettings
	err := m.db.QueryRow(
		`SELECT session_id, COALESCE(user_api_key, ''), COALESCE(model_override, ''), updated_at
		 FROM ai_chat_settings WHERE session_id = $1`,
		sessionID,
	).Scan(&s.SessionID, &s.UserAPIKey, &s.ModelOverride, &s.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return &AIChatSettings{SessionID: sessionID}, nil
		}
		return nil, fmt.Errorf("failed to get settings for session %s: %w", sessionID, err)
	}
	return &s, nil
}

// SaveSettings saves per-chat settings (API key, model override)
func (m *AIChatManager) SaveSettings(sessionID, apiKey, model string) error {
	_, err := m.db.Exec(
		`INSERT INTO ai_chat_settings (session_id, user_api_key, model_override, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (session_id) DO UPDATE SET
		 user_api_key = $2, model_override = $3, updated_at = NOW()`,
		sessionID, apiKey, model,
	)
	if err != nil {
		return fmt.Errorf("failed to save settings for session %s: %w", sessionID, err)
	}
	return nil
}

// UpdateSession updates session fields (model, system_prompt, active_agent_id, agent_mode)
func (m *AIChatManager) UpdateSession(sessionID, model, systemPrompt, activeAgentID, agentMode string) error {
	_, err := m.db.Exec(
		`UPDATE ai_chat_sessions SET
		 model = COALESCE(NULLIF($2, ''), model),
		 system_prompt = COALESCE(NULLIF($3, ''), system_prompt),
		 active_agent_id = COALESCE(NULLIF($4, ''), active_agent_id),
		 agent_mode = COALESCE(NULLIF($5, ''), agent_mode),
		 updated_at = NOW()
		 WHERE id = $1`,
		sessionID, model, systemPrompt, activeAgentID, agentMode,
	)
	return err
}

// GetOwnerID returns the user_id (owner) of a session
func (m *AIChatManager) GetOwnerID(sessionID string) string {
	var userID string
	err := m.db.QueryRow("SELECT user_id FROM ai_chat_sessions WHERE id = $1", sessionID).Scan(&userID)
	if err != nil {
		return ""
	}
	return userID
}
