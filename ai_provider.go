package main

// ai_provider.go — AgentProvider interface and shared types

import (
	"context"
)

// AgentProvider — any way to integrate an AI agent (API, subprocess, websocket, etc.)
type AgentProvider interface {
	// StreamChat streams the agent's response via channel
	StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error)

	// Capabilities returns what this provider supports
	Capabilities() AgentCapabilities

	// HealthCheck verifies the provider is available
	HealthCheck(ctx context.Context) error

	// Close releases resources
	Close() error
}

// StreamChunk is a single chunk from a streaming response
type StreamChunk struct {
	Content  string
	ToolCall *ToolCallRequestInput
	Done     bool
	Error    error
}

// ToolCallRequestInput — LLM requests to call a tool
type ToolCallRequestInput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// AIMessageInput — message for provider consumption
type AIMessageInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefInput — tool definition for provider
type ToolDefInput struct {
	Type     string         `json:"type"`
	Function ToolDefFuncInput `json:"function"`
}

type ToolDefFuncInput struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// AgentCapabilities — what a provider supports
type AgentCapabilities struct {
	SupportsImages    bool
	SupportsTools     bool
	SupportsStreaming bool
	MaxTokens         int
}
