package main

// ai_tool.go — Tool interface and types for AI agents

import (
	"context"
)

// Tool — any tool an AI agent can use
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON Schema
	Execute(ctx context.Context, args map[string]any) (string, error)
	RequiredRole() string // "user", "admin", "system"
}

// ToolDef — tool definition for LLM function calling
type ToolDef struct {
	Type     string      `json:"type"`
	Function ToolDefFunc `json:"function"`
}

type ToolDefFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
