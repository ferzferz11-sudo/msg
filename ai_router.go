package main

// ai_router.go — Hybrid router (heuristic + LLM fallback)

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// HybridRouter routes user messages to the best agent
type HybridRouter struct {
	db *sql.DB
}

// NewHybridRouter creates a new router
func NewHybridRouter(db *sql.DB) *HybridRouter {
	return &HybridRouter{db: db}
}

// Route determines which agent should handle a message
func (r *HybridRouter) Route(ctx context.Context, userID, message string, chat *AIChatV2) (string, error) {
	// 1. If chat has a bound agent, use it
	if chat.BoundAgentID != "" && chat.BindUntilMsg > 0 {
		return chat.BoundAgentID, nil
	}

	// 2. If chat has an explicit agent, use it
	if chat.AgentID != "" {
		return chat.AgentID, nil
	}

	// 3. Check keyword rules
	agentID, err := r.matchKeywordRules(ctx, message)
	if err == nil && agentID != "" {
		return agentID, nil
	}

	// 4. Default to assistant
	return "assistant", nil
}

func (r *HybridRouter) matchKeywordRules(ctx context.Context, message string) (string, error) {
	// Simple keyword matching
	lower := strings.ToLower(message)
	rules := []struct {
		keywords []string
		agentID  string
	}{
		{[]string{"code", "function", "bug", "debug", "refactor", "implement"}, "developer"},
		{[]string{"deploy", "server", "docker", "nginx", "systemd", "ssh"}, "devops"},
		{[]string{"architecture", "design", "scale", "microservice"}, "architect"},
		{[]string{"translate", "переведи", "перевод"}, "translator"},
		{[]string{"write", "story", "creative", "poem"}, "writer"},
		{[]string{"data", "analyze", "metrics", "chart", "report"}, "analyst"},
	}

	for _, rule := range rules {
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				return rule.agentID, nil
			}
		}
	}

	return "", fmt.Errorf("no keyword match")
}
