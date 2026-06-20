package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
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
// Реализации: OpenRouter, Hermes (локальный), любой другой
type LLMProvider interface {
	// StreamChat — стримит ответ LLM через канал
	StreamChat(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)

	// ModelID — идентификатор модели для логирования и маршрутизации
	ModelID() string

	// Close — освобождение ресурсов
	Close() error
}

// LLMRouter — маршрутизатор между провайдерами
type LLMRouter interface {
	// Route — выбирает провайдера и стримит ответ
	Route(ctx context.Context, modelHint string, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)

	// Register — регистрирует правило маршрутизации
	Register(rule RouteRule)
}

// RouteRule — правило маршрутизации
type RouteRule struct {
	ModelPrefix string      // "openrouter/" → OpenRouter, "local/" → Hermes, "" → default
	Provider    LLMProvider // провайдер
	Priority    int         // приоритет (выше = важнее)
}

// SimpleRouter — простая реализация LLMRouter
type SimpleRouter struct {
	mu    sync.RWMutex
	rules []RouteRule
	def   LLMProvider // default provider
}

func NewSimpleRouter(defaultProvider LLMProvider) *SimpleRouter {
	return &SimpleRouter{
		rules: make([]RouteRule, 0),
		def:   defaultProvider,
	}
}

func (r *SimpleRouter) Register(rule RouteRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, rule)
	// Сортируем по приоритету (выше = первый)
	for i := len(r.rules) - 1; i > 0; i-- {
		if r.rules[i].Priority > r.rules[i-1].Priority {
			r.rules[i], r.rules[i-1] = r.rules[i-1], r.rules[i]
		}
	}
}

func (r *SimpleRouter) Route(
	ctx context.Context,
	modelHint string,
	messages []Message,
	tools []ToolDef,
) (<-chan StreamChunk, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Ищем подходящее правило
	for _, rule := range r.rules {
		if rule.ModelPrefix == "" || strings.HasPrefix(modelHint, rule.ModelPrefix) {
			log.Printf("[Router] matched prefix=%q provider=%s", rule.ModelPrefix, rule.Provider.ModelID())
			return rule.Provider.StreamChat(ctx, messages, tools)
		}
	}

	// Fallback на default
	if r.def != nil {
		log.Printf("[Router] no match for hint=%q, using default=%s", modelHint, r.def.ModelID())
		return r.def.StreamChat(ctx, messages, tools)
	}

	return nil, fmt.Errorf("no provider found for model_hint=%q and no default", modelHint)
}
