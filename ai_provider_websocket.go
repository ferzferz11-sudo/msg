package main

// ai_provider_websocket.go — WebSocket provider for AI agents

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type webSocketProvider struct {
	url          string
	authHeader   string
	pingInterval time.Duration
	dialer       *websocket.Dialer
}

func newWebSocketProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	url, _ := config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("WebSocket URL is required")
	}
	authHeader, _ := config["auth_header"].(string)
	if authHeader == "" && apiKey != "" {
		authHeader = "Bearer " + apiKey
	}
	pingSec := 30
	if p, ok := config["ping_interval_seconds"].(float64); ok {
		pingSec = int(p)
	}
	return &webSocketProvider{
		url:          url,
		authHeader:   authHeader,
		pingInterval: time.Duration(pingSec) * time.Second,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}, nil
}

type wsRequest struct {
	Type     string           `json:"type"`
	Messages []AIMessageInput `json:"messages,omitempty"`
	Tools    []ToolDefInput   `json:"tools,omitempty"`
}

type wsResponse struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
	Done    bool   `json:"done"`
}

func (p *webSocketProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 64)

	// Set up headers for handshake
	header := http.Header{}
	if p.authHeader != "" {
		header.Set("Authorization", p.authHeader)
	}

	conn, _, err := p.dialer.DialContext(ctx, p.url, header)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Start ping goroutine
	pingCtx, pingCancel := context.WithCancel(ctx)
	var pingOnce sync.Once
	go func() {
		ticker := time.NewTicker(p.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					pingOnce.Do(func() { pingCancel() })
					return
				}
			}
		}
	}()

	// Send request
	req := wsRequest{
		Type:     "chat",
		Messages: messages,
		Tools:    tools,
	}
	if err := conn.WriteJSON(req); err != nil {
		pingCancel()
		conn.Close()
		close(ch)
		return nil, fmt.Errorf("WebSocket send failed: %w", err)
	}

	// Read responses
	go func() {
		defer close(ch)
		defer pingCancel()
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		for {
			var resp wsResponse
			if err := conn.ReadJSON(&resp); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					ch <- StreamChunk{Done: true}
					return
				}
				ch <- StreamChunk{Error: fmt.Errorf("WebSocket read error: %w", err), Done: true}
				return
			}

			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

			if resp.Error != "" {
				ch <- StreamChunk{Error: fmt.Errorf(resp.Error), Done: true}
				return
			}

			if resp.Content != "" {
				ch <- StreamChunk{Content: resp.Content}
			}

			if resp.Done {
				ch <- StreamChunk{Done: true}
				return
			}
		}
	}()

	return ch, nil
}

func (p *webSocketProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
		MaxTokens:         4096,
	}
}

func (p *webSocketProvider) HealthCheck(ctx context.Context) error {
	header := http.Header{}
	if p.authHeader != "" {
		header.Set("Authorization", p.authHeader)
	}
	conn, _, err := p.dialer.DialContext(ctx, p.url, header)
	if err != nil {
		return fmt.Errorf("WebSocket health check failed: %w", err)
	}
	conn.Close()
	return nil
}

func (p *webSocketProvider) Close() error { return nil }
