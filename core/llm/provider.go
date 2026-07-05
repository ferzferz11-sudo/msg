package llm

import (
	"context"
)

// Message — универсальное сообщение для LLM
type Message struct {
	Role    string   `json:"role"` // "user", "assistant", "system", "tool"
	Content string   `json:"content"`
	Images  [][]byte `json:"images,omitempty"` // raw image bytes (base64-encoded in JSON)
}

// ToolDef — описание инструмента для function calling
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ToolCall — сигнал от LLM о вызове функции
type ToolCall struct {
	ID       string       `json:"id"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// StreamChunk — чанк из стрима LLM
type StreamChunk struct {
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done"`
	Error    string    `json:"error,omitempty"`
}

// LLMProvider — интерфейс для любого LLM-провайдера
type LLMProvider interface {
	StreamChat(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)
	ModelID() string
	Close() error
}

// LLMRouter — маршрутизатор между провайдерами
type LLMRouter interface {
	Route(ctx context.Context, modelHint string, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)
}
