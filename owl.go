package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// OpenRouter API response structures
type oll struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// owlMessage represents a stored OWL chat message
type owlMessage struct {
	ID        int       `json:"id"`
	ChatID    string    `json:"chat_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// owlChatSettings stores per-chat user settings
type owlChatSettings struct {
	ChatID    string `json:"chat_id"`
	UserAPIKey string `json:"user_api_key"`
	Model     string `json:"model"`
}

// owlSessionManager replaces in-memory storage with DB-backed storage
type owlSessionManager struct {
	mu      sync.Mutex
	db      *sql.DB
	maxHist int
}

func newOwlSessionManager(db *sql.DB, maxHist int) *owlSessionManager {
	return &owlSessionManager{
		db:      db,
		maxHist: maxHist,
	}
}

func (s *owlSessionManager) getHistory(chatID string) []map[string]string {
	rows, err := s.db.Query(
		"SELECT role, content FROM owl_messages WHERE chat_id = $1 ORDER BY created_at ASC",
		chatID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var history []map[string]string
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err == nil {
			history = append(history, map[string]string{"role": role, "content": content})
		}
	}
	return history
}

func (s *owlSessionManager) addMessage(chatID, role, content string) {
	_, err := s.db.Exec(
		"INSERT INTO owl_messages (chat_id, role, content) VALUES ($1, $2, $3)",
		chatID, role, content,
	)
	if err != nil {
		log.Printf("owlSessionManager: failed to save message: %v", err)
	}
}

func (s *owlSessionManager) clear(chatID string) {
	_, _ = s.db.Exec("DELETE FROM owl_messages WHERE chat_id = $1", chatID)
}

func (s *owlSessionManager) getSettings(chatID string) owlChatSettings {
	var settings owlChatSettings
	err := s.db.QueryRow(
		"SELECT chat_id, COALESCE(user_api_key, ''), COALESCE(model, '') FROM owl_chat_settings WHERE chat_id = $1",
		chatID,
	).Scan(&settings.ChatID, &settings.UserAPIKey, &settings.Model)
	if err != nil {
		return owlChatSettings{ChatID: chatID}
	}
	return settings
}

func (s *owlSessionManager) saveSettings(chatID, apiKey, model string) {
	_, err := s.db.Exec(
		`INSERT INTO owl_chat_settings (chat_id, user_api_key, model, updated_at) 
		 VALUES ($1, $2, $3, NOW()) 
		 ON CONFLICT (chat_id) DO UPDATE SET user_api_key=$2, model=$3, updated_at=NOW()`,
		chatID, apiKey, model,
	)
	if err != nil {
		log.Printf("owlSessionManager: failed to save settings: %v", err)
	}
}

func (s *owlSessionManager) getOwlChats(userID string) []string {
	rows, err := s.db.Query(
		"SELECT id FROM chats WHERE id LIKE $1 AND type = 'owl' ORDER BY created_at DESC",
		"owl-"+userID+"-%",
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var chats []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			chats = append(chats, id)
		}
	}
	return chats
}

// callOpenRouter sends a message to OpenRouter and returns the response
func callOpenRouter(apiKey string, model string, systemPrompt string, messages []map[string]string) (string, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured")
	}

	if model == "" {
		model = os.Getenv("OPENROUTER_MODEL")
		if model == "" {
			model = "openrouter/auto"
		}
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": append([]map[string]string{{"role": "system", "content": systemPrompt}}, messages...),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://lavender-messenger.com")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenRouter returned %d: %s", resp.StatusCode, string(body))
	}

	var result oll
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in OpenRouter response")
	}

	return result.Choices[0].Message.Content, nil
}

// Rate limiter per user
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var valid []time.Time
	for _, t := range rl.requests[userID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[userID] = valid
		return false
	}

	rl.requests[userID] = append(valid, now)
	return true
}
