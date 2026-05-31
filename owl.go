package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// CallOpenRouter sends a message to OpenRouter and returns the response
func callOpenRouter(model string, systemPrompt string, messages []map[string]string) (string, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	if model == "" {
		model = os.Getenv("OPENROUTER_MODEL")
		if model == "" {
			model = "openrouter/owl-alpha"
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

	// Filter old requests
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

// OWL session context stored in memory (can be moved to DB later)
type owlSession struct {
	mu       sync.Mutex
	contexts map[string][]map[string]string // userID -> message history
	maxHist  int
}

func newOwlSession(maxHist int) *owlSession {
	return &owlSession{
		contexts: make(map[string][]map[string]string),
		maxHist:  maxHist,
	}
}

func (s *owlSession) getHistory(userID string) []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contexts[userID]
}

func (s *owlSession) addMessage(userID, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.contexts[userID]
	history = append(history, map[string]string{"role": role, "content": content})

	// Keep only last maxHist messages
	if len(history) > s.maxHist {
		history = history[len(history)-s.maxHist:]
	}

	s.contexts[userID] = history
}

func (s *owlSession) clear(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.contexts, userID)
}
