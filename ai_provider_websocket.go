package main

// ai_provider_websocket.go — WebSocket provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type webSocketProvider struct {
	url          string
	authHeader   string
	pingInterval time.Duration
	client       *http.Client
}

func newWebSocketProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	url, _ := config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("WebSocket URL is required")
	}
	authHeader, _ := config["auth_header"].(string)
	pingSec := 30
	if p, ok := config["ping_interval_seconds"].(float64); ok {
		pingSec = int(p)
	}
	return &webSocketProvider{
		url:          url,
		authHeader:   authHeader,
		pingInterval: time.Duration(pingSec) * time.Second,
		client:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (p *webSocketProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 64)

	// WebSocket implementation requires gorilla/websocket or nhooyr.io/websocket
	// For now, return an error — this will be implemented when the dependency is added
	go func() {
		defer close(ch)
		ch <- StreamChunk{Error: fmt.Errorf("WebSocket provider not yet implemented"), Done: true}
	}()

	return ch, nil
}

func (p *webSocketProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
		MaxTokens:         4096,
	}
}

func (p *webSocketProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *webSocketProvider) Close() error { return nil }

// placeholder for future WebSocket message types
type wsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Mu      sync.Mutex      `json:"-"`
}
