package main

// rate_limiter.go — Sliding window rate limiter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Global rate limiters (replacing owl.go globals)
var (
	owlRateLimiter     = NewRedisRateLimiter(10, time.Minute, "rl:owl:")
	freeTierRateLimiter = NewRedisRateLimiter(20, time.Hour, "rl:free:")
)

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

	// Filter existing timestamps
	rl.requests[userID] = filterTimestamps(rl.requests[userID], cutoff)

	// Count current valid timestamps
	count := len(rl.requests[userID])

	if count >= rl.limit {
		return false
	}

	// Add new timestamp
	rl.requests[userID] = append(rl.requests[userID], now)
	return true
}

func (rl *rateLimiter) cancel(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	ts := rl.requests[userID]
	if len(ts) > 0 {
		rl.requests[userID] = ts[:len(ts)-1]
	}
}

func (rl *rateLimiter) remaining(userID string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	count := 0
	for _, t := range rl.requests[userID] {
		if t.After(cutoff) {
			count++
		}
	}
	rl.requests[userID] = filterTimestamps(rl.requests[userID], cutoff)

	remaining := rl.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for userID, ts := range rl.requests {
		valid := filterTimestamps(ts, cutoff)
		if len(valid) == 0 {
			delete(rl.requests, userID)
		} else {
			rl.requests[userID] = valid
		}
	}
}

func filterTimestamps(ts []time.Time, cutoff time.Time) []time.Time {
	result := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// callOpenRouterContext — non-streaming OpenRouter call (stub for bot_commands.go)
func callOpenRouterContext(ctx context.Context, apiKey string, model string, systemPrompt string, messages []map[string]string) (string, error) {
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
		"stream":   false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://lavender-messenger.com")

	resp, err := openRouterClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenRouter returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}
